package main

import (
  "context"
  "log"
  "net/http"
  "os"
  "path/filepath"
  "time"

  "github.com/go-chi/chi/v5"

  "hrm/internal/auth"
  "hrm/internal/config"
  "hrm/internal/core"
  "hrm/internal/db"
  "hrm/internal/gdpr"
  "hrm/internal/leave"
  "hrm/internal/middleware"
  "hrm/internal/notifications"
  "hrm/internal/payroll"
  "hrm/internal/performance"
  "hrm/internal/reports"
)

func main() {
  cfg := config.Load()
  if cfg.DatabaseURL == "" {
    log.Fatal("DATABASE_URL is required")
  }

  ctx := context.Background()
  pool, err := db.Connect(ctx, cfg)
  if err != nil {
    log.Fatalf("db connect failed: %v", err)
  }
  defer pool.Close()

  if err := db.Migrate(ctx, pool, "migrations"); err != nil {
    log.Fatalf("migrations failed: %v", err)
  }

  if err := db.Seed(ctx, pool, cfg); err != nil {
    log.Fatalf("seed failed: %v", err)
  }

  coreStore := core.NewStore(pool)

  router := chi.NewRouter()
  router.Use(middleware.RequestID)
  router.Use(middleware.Logger)
  router.Use(middleware.Recoverer)
  router.Use(middleware.Auth(cfg.JWTSecret))

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
    authHandler := auth.NewHandler(pool, cfg.JWTSecret)
    r.Post("/auth/login", authHandler.HandleLogin)
    r.Post("/auth/logout", authHandler.HandleLogout)
    r.Post("/auth/request-reset", authHandler.HandleRequestReset)
    r.Post("/auth/reset", authHandler.HandleResetPassword)

    coreHandler := core.NewHandler(coreStore)
    coreHandler.RegisterRoutes(r)

    leaveHandler := leave.NewHandler(pool)
    leaveHandler.RegisterRoutes(r)

    payrollHandler := payroll.NewHandler(pool)
    payrollHandler.RegisterRoutes(r)

    performanceHandler := performance.NewHandler(pool)
    performanceHandler.RegisterRoutes(r)

    gdprHandler := gdpr.NewHandler(pool)
    gdprHandler.RegisterRoutes(r)

    reportsHandler := reports.NewHandler(pool)
    reportsHandler.RegisterRoutes(r)

    notificationsHandler := notifications.NewHandler(notifications.New(pool))
    notificationsHandler.RegisterRoutes(r)
  })

  router.Mount("/", spaHandler{staticPath: cfg.FrontendDir, indexPath: "index.html"})

  log.Printf("HRM server listening on %s", cfg.Addr)
  if err := http.ListenAndServe(cfg.Addr, router); err != nil {
    log.Fatalf("server failed: %v", err)
  }
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
