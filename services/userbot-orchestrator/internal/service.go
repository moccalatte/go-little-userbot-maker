package internal

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/sync/errgroup"

	"go-little-userbot-maker/pkg/config"
	"go-little-userbot-maker/pkg/logger"
	"go-little-userbot-maker/pkg/storage"
	orchHttp "go-little-userbot-maker/services/userbot-orchestrator/internal/delivery/http"
	"go-little-userbot-maker/services/userbot-orchestrator/internal/repository"
	"go-little-userbot-maker/services/userbot-orchestrator/internal/usecase"
)

// Service is the main orchestrator service. It wires up all the components.
type Service struct {
	cfg     config.OrchestratorConfig
	log     *logger.Logger
	usecase *usecase.SessionUsecase
	handler *orchHttp.Handler
}

// NewService creates and configures the orchestrator service and its dependencies.
func NewService(cfg config.OrchestratorConfig, log *logger.Logger, db *storage.Database) (*Service, error) {
	if log == nil {
		return nil, errors.New("logger is nil")
	}
	if cfg.SecretKey == "" {
		log.Warn("secret key is not set, using a default. This is insecure for production.")
	}

	// 1. Initialize Repositories
	sessionRepo := repository.NewSessionRepository(log, db)

	// 2. Initialize Usecases
	sessionUsecase := usecase.NewSessionUsecase(cfg, log, sessionRepo, cfg.SecretKey)

	// 3. Initialize Delivery Layer (HTTP Handler)
	httpHandler := orchHttp.NewHandler(log, sessionUsecase)

	return &Service{
		cfg:     cfg,
		log:     log,
		usecase: sessionUsecase,
		handler: httpHandler,
	}, nil
}

// Run starts the orchestrator service.
func (s *Service) Run(ctx context.Context) error {
	if err := s.usecase.Bootstrap(ctx); err != nil {
		s.log.Warn("failed to bootstrap sessions, some userbots may not start automatically: %v", err)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Timeout(60 * time.Second))
	router.Use(s.requestLogger)

	// Register all API routes from the delivery handler
	s.handler.RegisterRoutes(router)

	apiServer := &http.Server{
		Addr:              s.cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	g, ctx := errgroup.WithContext(ctx)

	// Start API server
	g.Go(func() error {
		s.log.Info("orchestrator API server starting at %s", s.cfg.ListenAddr)
		if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	// Handle graceful shutdown
	g.Go(func() error {
		<-ctx.Done()
		s.log.Info("shutting down servers")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = apiServer.Shutdown(shutdownCtx)
		s.usecase.StopAllWorkers()
		return nil
	})

	return g.Wait()
}

// requestLogger is a logging middleware for HTTP requests.
func (s *Service) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info("http request: method=%s path=%s status=%d latency=%s request_id=%s",
			r.Method,
			r.URL.Path,
			ww.Status(),
			time.Since(start),
			middleware.GetReqID(r.Context()))
	})
}