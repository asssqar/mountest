package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/mountest/backend/internal/auth"
	"github.com/mountest/backend/internal/config"
	"github.com/mountest/backend/internal/db"
	"github.com/mountest/backend/internal/handlers"
	"github.com/mountest/backend/internal/seed"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	if err := seed.EnsureAdmin(ctx, pool, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if cfg.SeedDemo {
		if err := seed.EnsureDemo(ctx, pool); err != nil {
			log.Fatalf("seed demo: %v", err)
		}
	}

	authSvc := auth.NewService(cfg.JWTSecret, cfg.CookieSecure)
	pub := &handlers.PublicHandler{Pool: pool}
	adm := &handlers.AdminHandler{Pool: pool, Auth: authSvc}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	if cfg.CORSOrigin != "" {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{cfg.CORSOrigin},
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Accept"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		// Публичные эндпоинты
		r.Get("/subjects", pub.ListSubjects)
		r.Get("/subjects/{id}/variants", pub.ListSubjectVariants)

		r.Post("/guest-sessions", pub.CreateGuest)
		r.Get("/guest-sessions/{id}", pub.GetGuest)

		r.Post("/attempts", pub.StartAttempt)
		r.Get("/attempts/{id}", pub.GetAttempt)
		r.Put("/attempts/{id}/answer", pub.SaveAnswer)
		r.Post("/attempts/{id}/finish", pub.FinishAttempt)
		r.Get("/attempts/{id}/result", pub.GetResult)

		// Админ
		r.Post("/admin/login", adm.Login)
		r.Post("/admin/logout", adm.Logout)
		r.Group(func(r chi.Router) {
			r.Use(authSvc.Middleware)
			r.Get("/admin/me", adm.Me)

			r.Get("/admin/subjects", adm.ListSubjects)
			r.Post("/admin/subjects", adm.CreateSubject)
			r.Put("/admin/subjects/{id}", adm.UpdateSubject)
			r.Delete("/admin/subjects/{id}", adm.DeleteSubject)

			r.Get("/admin/variants", adm.ListVariants)
			r.Post("/admin/variants", adm.CreateVariant)
			r.Put("/admin/variants/{id}", adm.UpdateVariant)
			r.Delete("/admin/variants/{id}", adm.DeleteVariant)

			r.Get("/admin/questions", adm.ListQuestions)
			r.Get("/admin/questions/{id}", adm.GetQuestion)
			r.Post("/admin/questions", adm.CreateQuestion)
			r.Put("/admin/questions/{id}", adm.UpdateQuestion)
			r.Delete("/admin/questions/{id}", adm.DeleteQuestion)
		})
	})

	addr := ":" + cfg.AppPort
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("MounTest backend listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
