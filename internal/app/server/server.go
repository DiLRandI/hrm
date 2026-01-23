package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/audit"
	authdomain "hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/domain/gdpr"
	"hrm/internal/domain/leave"
	"hrm/internal/domain/notifications"
	"hrm/internal/domain/payroll"
	"hrm/internal/domain/performance"
	"hrm/internal/domain/reports"
	"hrm/internal/platform/config"
	cryptoutil "hrm/internal/platform/crypto"
	"hrm/internal/platform/db"
	"hrm/internal/platform/email"
	"hrm/internal/platform/jobs"
	"hrm/internal/platform/metrics"
	"hrm/internal/transport/http/api"
	audithandler "hrm/internal/transport/http/handlers/audit"
	authhandler "hrm/internal/transport/http/handlers/auth"
	corehandler "hrm/internal/transport/http/handlers/core"
	gdprhandler "hrm/internal/transport/http/handlers/gdpr"
	leavehandler "hrm/internal/transport/http/handlers/leave"
	notificationshandler "hrm/internal/transport/http/handlers/notifications"
	payrollhandler "hrm/internal/transport/http/handlers/payroll"
	performancehandler "hrm/internal/transport/http/handlers/performance"
	reportshandler "hrm/internal/transport/http/handlers/reports"
	"hrm/internal/transport/http/middleware"
)

type App struct {
	Config  config.Config
	DB      *db.Pool
	Router  http.Handler
	Stop    context.CancelFunc
	Jobs    *jobs.Service
	Metrics *metrics.Collector
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.RunMigrations {
		if err := db.Migrate(ctx, pool, "migrations"); err != nil {
			pool.Close()
			return nil, err
		}
	}

	if cfg.RunSeed {
		if err := db.Seed(ctx, pool, cfg); err != nil {
			pool.Close()
			return nil, err
		}
	}

	cryptoSvc, err := cryptoutil.New(cfg.DataEncryptionKey)
	if err != nil {
		pool.Close()
		return nil, err
	}

	coreStore := core.NewStore(pool, cryptoSvc)
	mailer := email.New(cfg)
	notifySvc := notifications.New(pool, mailer)
	notifySvc.DefaultFrom = cfg.EmailFrom
	jobsSvc := jobs.New(pool, cfg)
	metricsCollector := metrics.New()
	router := buildRouter(cfg, pool, coreStore, cryptoSvc, notifySvc, jobsSvc, metricsCollector)

	return &App{
		Config:  cfg,
		DB:      pool,
		Router:  router,
		Jobs:    jobsSvc,
		Metrics: metricsCollector,
	}, nil
}

func (a *App) Close() {
	if a.DB != nil {
		a.DB.Close()
	}
}

func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              a.Config.Addr,
		Handler:           a.Router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if a.Jobs != nil {
		a.Jobs.Start(ctx)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func buildRouter(cfg config.Config, pool *db.Pool, coreStore *core.Store, cryptoSvc *cryptoutil.Service, notifySvc *notifications.Service, jobsSvc *jobs.Service, metricsCollector *metrics.Collector) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger(metricsCollector))
	router.Use(middleware.Recoverer)
	router.Use(middleware.SecureHeaders(cfg.Environment == "production"))
	router.Use(middleware.Auth(cfg.JWTSecret, pool))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			slog.Warn("healthz write failed", "err", err)
		}
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ready")); err != nil {
			slog.Warn("readyz write failed", "err", err)
		}
	})

	if cfg.MetricsEnabled {
		router.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			api.Success(w, metricsCollector.Snapshot(), middleware.GetRequestID(r.Context()))
		})
	}

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.BodyLimit(cfg.MaxBodyBytes))
		authService := authdomain.NewService(authdomain.NewStore(pool))
		authHandler := authhandler.NewHandler(authService, cfg.JWTSecret, cryptoSvc)
		r.With(middleware.RateLimit(cfg.RateLimitPerMinute, time.Minute)).Post("/auth/login", authHandler.HandleLogin)
		r.Post("/auth/logout", authHandler.HandleLogout)
		r.Post("/auth/refresh", authHandler.HandleRefresh)
		r.With(middleware.RateLimit(cfg.RateLimitPerMinute, time.Minute)).Post("/auth/request-reset", authHandler.HandleRequestReset)
		r.Post("/auth/reset", authHandler.HandleResetPassword)
		r.Post("/auth/mfa/setup", authHandler.HandleMFASetup)
		r.Post("/auth/mfa/enable", authHandler.HandleMFAEnable)
		r.Post("/auth/mfa/disable", authHandler.HandleMFADisable)

		auditSvc := audit.New(pool)
		coreService := core.NewService(coreStore)
		coreHandler := corehandler.NewHandler(coreService, auditSvc)
		coreHandler.RegisterRoutes(r)

		auditHandler := audithandler.NewHandler(auditSvc, coreStore)
		auditHandler.RegisterRoutes(r)

		leaveService := leave.NewService(leave.NewStore(pool), coreStore)
		leaveHandler := leavehandler.NewHandler(leaveService, coreStore, notifySvc, auditSvc, jobsSvc)
		leaveHandler.RegisterRoutes(r)

		payrollService := payroll.NewService(payroll.NewStore(pool), cryptoSvc)
		payrollHandler := payrollhandler.NewHandler(payrollService, coreStore, cryptoSvc, notifySvc, jobsSvc, auditSvc)
		payrollHandler.RegisterRoutes(r)

		performanceService := performance.NewService(performance.NewStore(pool))
		performanceHandler := performancehandler.NewHandler(performanceService, coreStore, notifySvc, auditSvc)
		performanceHandler.RegisterRoutes(r)

		gdprService := gdpr.NewService(gdpr.NewStore(pool), coreStore, cryptoSvc)
		gdprHandler := gdprhandler.NewHandler(gdprService, coreStore, cryptoSvc, jobsSvc, auditSvc)
		gdprHandler.RegisterRoutes(r)

		reportsService := reports.NewService(reports.NewStore(pool))
		reportsHandler := reportshandler.NewHandler(reportsService, coreStore)
		reportsHandler.RegisterRoutes(r)

		notificationsHandler := notificationshandler.NewHandler(notifySvc)
		notificationsHandler.RegisterRoutes(r)
	})

	router.Mount("/", spaHandler{staticPath: cfg.FrontendDir, indexPath: "index.html"})
	return router
}

type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(h.staticPath, r.URL.Path)
	_, err := os.Stat(path)
	if err == nil {
		http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
		return
	}

	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}

	http.NotFound(w, r)
}
