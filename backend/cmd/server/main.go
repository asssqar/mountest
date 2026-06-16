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
	"github.com/go-chi/httprate"

	"github.com/mountest/backend/internal/auth"
	"github.com/mountest/backend/internal/config"
	"github.com/mountest/backend/internal/db"
	"github.com/mountest/backend/internal/handlers"
	"github.com/mountest/backend/internal/seed"
)

func main() {
	cfg := config.Load()

	// Fail-fast: не даём подняться в проде с дефолтными секретами.
	// CookieSecure=true — наш индикатор прод-окружения.
	if cfg.CookieSecure {
		if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me" ||
			cfg.JWTSecret == "change-me-in-production-please-32+chars" ||
			len(cfg.JWTSecret) < 32 {
			log.Fatal("JWT_SECRET must be a strong random value in production (use: openssl rand -hex 32)")
		}
		if cfg.AdminPassword == "" || cfg.AdminPassword == "admin123" {
			log.Fatal("ADMIN_PASSWORD must be set to a strong value in production")
		}
	}

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

	if err := seed.EnsureAdmin(ctx, pool, cfg.AdminUsername, cfg.AdminPassword, auth.RoleSuperadmin); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if cfg.SeedDemo {
		if err := seed.EnsureDemo(ctx, pool); err != nil {
			log.Fatalf("seed demo: %v", err)
		}
	}

	authSvc := auth.NewService(cfg.JWTSecret, cfg.CookieSecure)
	pub := &handlers.PublicHandler{
		Pool:          pool,
		AttemptSecret: handlers.DeriveAttemptSecret(cfg.JWTSecret),
	}
	adm := &handlers.AdminHandler{Pool: pool, Auth: authSvc}
	usr := &handlers.UsersHandler{Pool: pool}
	upl := &handlers.UploadHandler{Dir: cfg.UploadDir}
	if err := upl.EnsureDir(); err != nil {
		log.Fatalf("ensure upload dir: %v", err)
	}
	log.Printf("upload directory: %s", cfg.UploadDir)

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
		// ---------- public reads (без лимита, кешируется CDN/Caddy) ----------
		r.Get("/subjects", pub.ListSubjects)
		r.Get("/subjects/{id}/variants", pub.ListSubjectVariants)
		r.Get("/guest-sessions/{id}", pub.GetGuest)
		r.Get("/attempts/{id}", pub.GetAttempt)
		r.Get("/attempts/{id}/result", pub.GetResult)

		// Картинки вопросов отдаём публично: их видят ученики во время попытки.
		r.Get("/uploads/{name}", upl.Serve)

		// ---------- public writes: грубый rate-limit, чтобы не было спам-ботов ----------
		// 30 регистраций гостей и 30 стартов попыток с одного IP в минуту — с запасом для класса.
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(30, time.Minute))
			r.Post("/guest-sessions", pub.CreateGuest)
			r.Post("/attempts", pub.StartAttempt)
			r.Post("/attempts/{id}/finish", pub.FinishAttempt)
		})

		// Сохранение ответа триггерится автоматически на каждый клик — лимит щедрее.
		// 600 в минуту = 10 в секунду на IP, выше реально не нужно (debounce на фронте отсутствует).
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(600, time.Minute))
			r.Put("/attempts/{id}/answer", pub.SaveAnswer)
		})

		// ---------- admin: login (с rate-limit) ----------
		r.Group(func(r chi.Router) {
			// 10 попыток в минуту с одного IP — этого достаточно для людей и режет ботов.
			r.Use(httprate.LimitByIP(10, time.Minute))
			r.Post("/admin/login", adm.Login)
		})
		r.Post("/admin/logout", adm.Logout)

		// ---------- admin: всё остальное (нужна авторизация) ----------
		r.Group(func(r chi.Router) {
			r.Use(authSvc.Middleware)
			r.Get("/admin/me", adm.Me)

			// Subjects: список открыт всем админам, мутации — только superadmin.
			r.Get("/admin/subjects", adm.ListSubjects)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireSuperadmin)
				r.Post("/admin/subjects", adm.CreateSubject)
				r.Put("/admin/subjects/{id}", adm.UpdateSubject)
				r.Delete("/admin/subjects/{id}", adm.DeleteSubject)
			})

			// Variants: editor видит и редактирует только свои (ownership внутри хендлеров).
			r.Get("/admin/variants", adm.ListVariants)
			r.Post("/admin/variants", adm.CreateVariant)
			r.Put("/admin/variants/{id}", adm.UpdateVariant)
			r.Delete("/admin/variants/{id}", adm.DeleteVariant)
			r.Post("/admin/variants/{id}/publish", adm.SetVariantPublished)
			r.Post("/admin/variants/{id}/import-questions", adm.ImportQuestions)

			// Questions: ownership через variant внутри хендлеров.
			r.Get("/admin/questions", adm.ListQuestions)
			r.Get("/admin/questions/{id}", adm.GetQuestion)
			r.Post("/admin/questions", adm.CreateQuestion)
			r.Put("/admin/questions/{id}", adm.UpdateQuestion)
			r.Delete("/admin/questions/{id}", adm.DeleteQuestion)

			// Картинки: загружать может любой залогиненный админ.
			r.Post("/admin/upload", upl.Upload)

			// Users: только superadmin.
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireSuperadmin)
				r.Get("/admin/users", usr.List)
				r.Post("/admin/users", usr.Create)
				r.Post("/admin/users/{id}/reset-password", usr.ResetPassword)
				r.Delete("/admin/users/{id}", usr.Delete)
			})
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
