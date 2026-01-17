package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/core"
	"hrm/internal/domain/notifications"
	"hrm/internal/platform/config"
	"hrm/internal/platform/db"
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
	Config config.Config
	DB     *db.Pool
	Router http.Handler
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

	coreStore := core.NewStore(pool)
	router := buildRouter(cfg, pool, coreStore)

	return &App{
		Config: cfg,
		DB:     pool,
		Router: router,
	}, nil
}

func (a *App) Close() {
	if a.DB != nil {
		a.DB.Close()
	}
}

func (a *App) Run() error {
	srv := &http.Server{
		Addr:              a.Config.Addr,
		Handler:           a.Router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return srv.ListenAndServe()
}

func buildRouter(cfg config.Config, pool *db.Pool, coreStore *core.Store) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.SecureHeaders(cfg.Environment == "production"))
	router.Use(middleware.Auth(cfg.JWTSecret, pool))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.BodyLimit(cfg.MaxBodyBytes))
		authHandler := authhandler.NewHandler(pool, cfg.JWTSecret)
		r.With(middleware.RateLimit(cfg.RateLimitPerMinute, time.Minute)).Post("/auth/login", authHandler.HandleLogin)
		r.Post("/auth/logout", authHandler.HandleLogout)
		r.With(middleware.RateLimit(cfg.RateLimitPerMinute, time.Minute)).Post("/auth/request-reset", authHandler.HandleRequestReset)
		r.Post("/auth/reset", authHandler.HandleResetPassword)

		coreHandler := corehandler.NewHandler(coreStore)
		coreHandler.RegisterRoutes(r)

		leaveHandler := leavehandler.NewHandler(pool, coreStore)
		leaveHandler.RegisterRoutes(r)

		payrollHandler := payrollhandler.NewHandler(pool, coreStore)
		payrollHandler.RegisterRoutes(r)

		performanceHandler := performancehandler.NewHandler(pool, coreStore)
		performanceHandler.RegisterRoutes(r)

		gdprHandler := gdprhandler.NewHandler(pool, coreStore)
		gdprHandler.RegisterRoutes(r)

		reportsHandler := reportshandler.NewHandler(pool, coreStore)
		reportsHandler.RegisterRoutes(r)

		notificationsHandler := notificationshandler.NewHandler(notifications.New(pool))
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
