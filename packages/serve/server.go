package serve

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/history"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/mock"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/proxy"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/stress"
)

// Server is the hitspec serve HTTP server.
type Server struct {
	config       *ServeConfig
	hub          *Hub
	history      *History
	historyStore *history.Store
	fileConfig   *config.Config
	configPath   string // resolved path to hitspec.yaml for write-back
	logger       *slog.Logger

	// Config state protected by configMu (RWMutex for read-heavy access)
	configMu sync.RWMutex

	// Mutable state protected by mu
	mu               sync.Mutex
	stressRunner     *stress.Runner
	stressCancel     context.CancelFunc
	lastStressResult *StressResultDTO
	mockServer       *mock.Server
	mockCancel       context.CancelFunc
	mockPort         int
	recorder         *proxy.Recorder
	recorderCancel   context.CancelFunc
	recorderPort     int
	recorderTarget   string

	ctx    context.Context
	cancel context.CancelFunc

	// boundPort is the port the server listens on; set by buildHandler before
	// any request is served.
	boundPort int

	// Watcher suppression for server-initiated writes
	watchSuppress *watchSuppressor

	// Version info set from CLI
	Version   string
	BuildTime string
}

// NewServer creates a new Server.
func NewServer(opts ...Option) *Server {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	absWorkDir, err := filepath.Abs(cfg.WorkDir)
	if err == nil {
		cfg.WorkDir = absWorkDir
	}

	// Load hitspec.yaml config and track the resolved path for write-back
	var configPath string
	var fileConfig *config.Config
	if cfg.ConfigPath != "" {
		fileConfig, _ = config.LoadConfig(cfg.ConfigPath)
		configPath = cfg.ConfigPath
	}
	if fileConfig == nil {
		configPath = config.FindConfigPath(cfg.WorkDir)
		fileConfig, _ = config.FindAndLoadConfig(cfg.WorkDir)
	}

	// An explicit --env (cfg.Env != "") is honored; only when unset do we fall
	// back to the config's defaultEnvironment, then "dev". The old `cfg.Env ==
	// "dev"` check treated an explicit `--env dev` the same as the default and
	// silently overrode it with defaultEnvironment.
	if cfg.Env == "" {
		if fileConfig != nil && fileConfig.DefaultEnvironment != "" {
			cfg.Env = fileConfig.DefaultEnvironment
		} else {
			cfg.Env = "dev"
		}
	}

	s := &Server{
		config:     cfg,
		hub:        NewHub(),
		history:    NewHistory(),
		fileConfig: fileConfig,
		configPath: configPath,
	}

	s.logger = newLogger(cfg)

	// Open persistent history database
	dbPath := cfg.HistoryDBPath
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dir := filepath.Join(home, ".hitspec")
			_ = os.MkdirAll(dir, 0o755)
			dbPath = filepath.Join(dir, "history.db")
		}
	}
	if dbPath != "" {
		_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
		store, err := history.NewStore(dbPath)
		if err != nil {
			s.logger.Warn("failed to open history database", "error", err, "path", dbPath)
		} else {
			s.historyStore = store
		}
	}

	return s
}

// ensureAPIToken generates a random API token when the server is not bound to a
// loopback address, or when any --allow-* flag is set, and none was configured.
// The token is printed once to w. Without it such a server would be open to
// anything that can reach it.
func (s *Server) ensureAPIToken(w io.Writer) error {
	if s.config.APIToken != "" {
		return nil
	}
	if isLoopbackBind(s.config.Host) && !s.config.AllowShell && !s.config.AllowDB {
		return nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// No entropy: refuse to run unauthenticated rather than fall back.
		return fmt.Errorf("generating API token: %w", err)
	}
	s.config.APIToken = hex.EncodeToString(buf)
	fmt.Fprintf(w, "hitspec: no --api-token / HITSPEC_API_TOKEN set, but this server is not loopback-only or has --allow-* enabled.\n"+
		"hitspec: generated API token (shown once): %s\n"+
		"hitspec: send it as 'Authorization: Bearer <token>' (or '?token=<token>' for WebSockets); set --api-token to choose your own.\n", s.config.APIToken)
	return nil
}

// buildHandler assembles the middleware chain around the routes. port is the
// port the server actually listens on. Order, outermost first: panic recovery,
// logging, Host check (loopback binds only), Origin/CORS (answers preflights
// before auth, since browsers never send credentials on them), API token,
// read-only gate.
func (s *Server) buildHandler(port int) http.Handler {
	s.boundPort = port
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	mws := []Middleware{recoveryMiddleware(s.logger), loggingMiddleware(s.logger)}
	if isLoopbackBind(s.config.Host) {
		mws = append(mws, hostCheckMiddleware(port))
	}
	mws = append(mws,
		corsMiddleware(s.config.CORS, port),
		authTokenMiddleware(s.config.APIToken),
		readOnlyMiddleware(s.config.ReadOnly),
	)
	return chain(mux, mws...)
}

// Start runs the server and blocks until shutdown.
func (s *Server) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			s.logger.Info("shutting down")
			s.cancel()
		case <-s.ctx.Done():
		}
	}()

	// Start file watcher
	if s.config.Watch {
		s.startWatcher()
	}

	// A server that is reachable beyond the local machine, or that can run
	// shell commands / reach databases, must not be open.
	if err := s.ensureAPIToken(os.Stderr); err != nil {
		return err
	}

	// Bind first so the middleware knows the real listening port (Host/Origin
	// checks), even for --port 0.
	ln, err := net.Listen("tcp", net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)))
	if err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	defer ln.Close()
	port := s.config.Port
	if ta, ok := ln.Addr().(*net.TCPAddr); ok {
		port = ta.Port
	}
	addr := net.JoinHostPort(s.config.Host, strconv.Itoa(port))

	server := &http.Server{
		Addr:         addr,
		Handler:      s.buildHandler(port),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
		BaseContext: func(l net.Listener) context.Context {
			return s.ctx
		},
	}

	// Graceful shutdown goroutine
	go func() {
		<-s.ctx.Done()
		if s.historyStore != nil {
			_ = s.historyStore.Close()
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	s.logger.Info("server started", "addr", addr)
	s.logger.Info("workspace configured", "path", s.config.WorkDir)

	files, _ := collectHitspecFiles(s.config.WorkDir)
	s.logger.Info("hitspec files discovered", "count", len(files))

	s.logger.Info("API available", "url", fmt.Sprintf("http://%s/api/v1/", addr))

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
