package serve

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/packages/history"
	"github.com/abdul-hamid-achik/hitspec/packages/mock"
	"github.com/abdul-hamid-achik/hitspec/packages/proxy"
	"github.com/abdul-hamid-achik/hitspec/packages/stress"
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
	fileConfig, _ := config.LoadConfig(cfg.ConfigPath)
	if fileConfig != nil && cfg.ConfigPath != "" {
		configPath = cfg.ConfigPath
	}
	if fileConfig == nil {
		configPath = config.FindConfigPath(cfg.WorkDir)
		fileConfig, _ = config.FindAndLoadConfig(cfg.WorkDir)
	}

	// Use defaultEnvironment from config if user didn't explicitly set --env
	if fileConfig != nil && fileConfig.DefaultEnvironment != "" && cfg.Env == "dev" {
		cfg.Env = fileConfig.DefaultEnvironment
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

	// Build HTTP handler
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	handler := chain(mux,
		recoveryMiddleware(s.logger),
		corsMiddleware(s.config.CORS),
		loggingMiddleware(s.logger),
		readOnlyMiddleware(s.config.ReadOnly),
	)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
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

	if !s.config.APIOnly {
		s.logger.Info("UI available", "url", fmt.Sprintf("http://%s", addr))
	}
	s.logger.Info("API available", "url", fmt.Sprintf("http://%s/api/v1/", addr))

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
