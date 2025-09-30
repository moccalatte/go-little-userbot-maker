package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"go-little-userbot-maker/internal/config"
	"go-little-userbot-maker/internal/storage"
)

type Service struct {
	cfg      config.OrchestratorConfig
	log      *zap.Logger
	db       *storage.Database
	redis    *storage.Redis
	manager  *SessionManager
	commands *Registry
	metrics  *Metrics
}

func NewService(cfg config.OrchestratorConfig, log *zap.Logger, db *storage.Database, redis *storage.Redis) (*Service, error) {
	if log == nil {
		return nil, errors.New("logger is nil")
	}
	if cfg.SecretKey == "" {
		log.Warn("secret key empty, using derived default")
	}
	manager := NewSessionManager(cfg, log, db)
	registry := NewRegistry(log)
	metrics := NewMetrics()

	return &Service{
		cfg:      cfg,
		log:      log,
		db:       db,
		redis:    redis,
		manager:  manager,
		commands: registry,
		metrics:  metrics,
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	if err := s.manager.Bootstrap(ctx); err != nil {
		s.log.Warn("bootstrap sessions", zap.Error(err))
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Timeout(60 * time.Second))
	router.Use(s.requestLogger)

	router.Get("/healthz", s.handleHealth)
	router.Post("/sessions", s.handleCreateSession)
	router.Delete("/sessions/{telegramID}", s.handleDeleteSession)
	router.Get("/stats/database", s.handleStats)
	router.Patch("/features/{feature}/{telegramID}", s.handlePatchFeature)

	apiServer := &http.Server{
		Addr:              s.cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	metricMux := http.NewServeMux()
	metricMux.Handle("/metrics", s.metrics.Handler())
	metricsServer := &http.Server{
		Addr:              s.cfg.MetricsAddr,
		Handler:           metricMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		s.log.Info("orchestrator listening", zap.String("addr", s.cfg.ListenAddr))
		if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	if s.cfg.MetricsAddr != "" {
		g.Go(func() error {
			s.log.Info("metrics listening", zap.String("addr", s.cfg.MetricsAddr))
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		})
	}

	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = apiServer.Shutdown(shutdownCtx)
		_ = metricsServer.Shutdown(shutdownCtx)
		s.manager.Stop()
		return nil
	})

	return g.Wait()
}

func (s *Service) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info("http_request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", ww.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("request_id", middleware.GetReqID(r.Context())))
	})
}
