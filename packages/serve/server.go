package serve

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/packages/mock"
	"github.com/abdul-hamid-achik/hitspec/packages/stress"
)

// Server is the hitspec serve HTTP server.
type Server struct {
	config  *ServeConfig
	hub     *Hub
	history *History
	fileConfig *config.Config

	// Mutable state protected by mu
	mu          sync.Mutex
	stressRunner *stress.Runner
	stressCancel context.CancelFunc
	mockServer   *mock.Server
	mockCancel   context.CancelFunc

	ctx    context.Context
	cancel context.CancelFunc

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

	// Load hitspec.yaml config
	fileConfig, _ := config.LoadConfig(cfg.ConfigPath)
	if fileConfig == nil {
		fileConfig, _ = config.FindAndLoadConfig(cfg.WorkDir)
	}

	// Use defaultEnvironment from config if user didn't explicitly set --env
	if fileConfig != nil && fileConfig.DefaultEnvironment != "" && cfg.Env == "dev" {
		cfg.Env = fileConfig.DefaultEnvironment
	}

	return &Server{
		config:     cfg,
		hub:        NewHub(),
		history:    NewHistory(),
		fileConfig: fileConfig,
	}
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
			log.Println("Shutting down...")
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
		recoveryMiddleware(),
		corsMiddleware(s.config.CORS),
		loggingMiddleware(s.config.Verbose),
		readOnlyMiddleware(s.config.ReadOnly),
	)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		BaseContext: func(l net.Listener) context.Context {
			return s.ctx
		},
	}

	// Graceful shutdown goroutine
	go func() {
		<-s.ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("hitspec serve running on http://%s", addr)
	log.Printf("  Workspace: %s", s.config.WorkDir)

	files, _ := collectHitspecFiles(s.config.WorkDir)
	log.Printf("  Files: %d .http/.hitspec files found", len(files))

	if !s.config.APIOnly {
		log.Printf("  UI: http://%s", addr)
	}
	log.Printf("  API: http://%s/api/v1/", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
