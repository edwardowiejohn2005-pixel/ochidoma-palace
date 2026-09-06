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
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"github.com/ochidoma/platform/internal/config"
	"github.com/ochidoma/platform/internal/db"
	adminhandlers "github.com/ochidoma/platform/internal/handlers/admin"
	authhandlers "github.com/ochidoma/platform/internal/handlers/auth"
	publichandlers "github.com/ochidoma/platform/internal/handlers/public"
	appmiddleware "github.com/ochidoma/platform/internal/middleware"
	"github.com/ochidoma/platform/internal/repository"
)

func main() {
	// .env is only loaded in development; in production, real env vars are set
	// by the host (systemd unit, Railway/Render dashboard, etc.) — never committed.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.RunMigrations(cfg.DatabaseURL, "migrations"); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer pool.Close()

	decreeRepo := repository.NewDecreeRepo(pool)
	decreeHandler := publichandlers.NewDecreeHandler(decreeRepo)
	decreeAdminHandler := adminhandlers.NewDecreeHandler(decreeRepo)

	articleRepo := repository.NewArticleRepo(pool)
	articleHandler := publichandlers.NewArticleHandler(articleRepo)
	articleAdminHandler := adminhandlers.NewArticleHandler(articleRepo)

	foodRepo := repository.NewFoodRepo(pool)
	foodHandler := publichandlers.NewFoodHandler(foodRepo)
	foodAdminHandler := adminhandlers.NewFoodHandler(foodRepo)

	eventRepo := repository.NewEventRepo(pool)
	eventHandler := publichandlers.NewEventHandler(eventRepo)
	eventAdminHandler := adminhandlers.NewEventHandler(eventRepo)

	announcementRepo := repository.NewAnnouncementRepo(pool)
	announcementHandler := publichandlers.NewAnnouncementHandler(announcementRepo)
	announcementAdminHandler := adminhandlers.NewAnnouncementHandler(announcementRepo)

	userRepo := repository.NewUserRepo(pool)
	authHandler := authhandlers.NewHandler(
		userRepo, cfg.JWTAccessSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL,
		cfg.Env == "production", // secure cookies only over real HTTPS
	)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(secureHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/decrees", func(r chi.Router) {
		r.Get("/", decreeHandler.List)
		r.Get("/{decreeNumber}", decreeHandler.GetByNumber)
	})

	r.Route("/api/articles", func(r chi.Router) {
		r.Get("/", articleHandler.List) // ?section=heritage|history&category_id=1
		r.Get("/{slug}", articleHandler.GetBySlug)
	})

	r.Route("/api/foods", func(r chi.Router) {
		r.Get("/", foodHandler.List)
		r.Get("/{slug}", foodHandler.GetBySlug)
	})

	r.Route("/api/events", func(r chi.Router) {
		r.Get("/", eventHandler.List) // ?status=upcoming|ongoing|completed
		r.Get("/{slug}", eventHandler.GetBySlug)
	})

	r.Route("/api/announcements", func(r chi.Router) {
		r.Get("/", announcementHandler.List)
		r.Get("/{id}", announcementHandler.GetByID)
	})

	r.Route("/api/auth", func(r chi.Router) {
		// 10 attempts/minute per IP, burst of 5 — login only. Refresh/logout
		// are token-gated already so they don't need the same throttle.
		r.With(appmiddleware.RateLimitByIP(10, 5)).Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
	})

	// Every /api/admin/* route requires a valid access token. Individual
	// routes layer RequireRole on top, matching the role table from the spec:
	//   super_admin        — everything
	//   palace_editor      — create/edit drafts, submit for approval
	//   palace_publisher   — approve/publish/archive official content
	//   cultural_editor    — manage heritage/history/food/events content end-to-end
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(appmiddleware.RequireAuth(cfg.JWTAccessSecret))

		r.Get("/whoami", func(w http.ResponseWriter, r *http.Request) {
			userID, _ := appmiddleware.UserIDFromContext(r.Context())
			role, _ := appmiddleware.RoleFromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"user_id":"` + userID.String() + `","role":"` + role + `"}`))
		})

		editorRoles := appmiddleware.RequireRole("super_admin", "palace_editor", "cultural_editor")
		culturalRoles := appmiddleware.RequireRole("super_admin", "cultural_editor")
		publisherRoles := appmiddleware.RequireRole("super_admin", "palace_publisher")

		// Articles/foods/events: cultural_editor owns this domain end-to-end,
		// including publishing — these aren't official Palace decrees, so they
		// don't need palace_publisher sign-off.
		r.Route("/articles", func(r chi.Router) {
			r.With(editorRoles).Get("/", articleAdminHandler.List)
			r.With(editorRoles).Post("/", articleAdminHandler.Create)
			r.With(editorRoles).Put("/{id}", articleAdminHandler.Update)
			r.With(culturalRoles).Post("/{id}/status", articleAdminHandler.SetStatus)
			r.With(culturalRoles).Delete("/{id}", articleAdminHandler.Delete)
		})

		r.Route("/foods", func(r chi.Router) {
			r.With(editorRoles).Get("/", foodAdminHandler.List)
			r.With(editorRoles).Post("/", foodAdminHandler.Create)
			r.With(editorRoles).Put("/{id}", foodAdminHandler.Update)
			r.With(culturalRoles).Post("/{id}/status", foodAdminHandler.SetStatus)
			r.With(culturalRoles).Delete("/{id}", foodAdminHandler.Delete)
		})

		r.Route("/events", func(r chi.Router) {
			r.With(editorRoles).Get("/", eventAdminHandler.List)
			r.With(editorRoles).Post("/", eventAdminHandler.Create)
			r.With(editorRoles).Put("/{id}", eventAdminHandler.Update)
			r.With(editorRoles).Post("/{id}/status", eventAdminHandler.SetStatus)
			r.With(editorRoles).Delete("/{id}", eventAdminHandler.Delete)
		})

		// Announcements: create/submit by editors, but approve/publish/archive
		// is palace_publisher/super_admin only — matches the spec exactly.
		r.Route("/announcements", func(r chi.Router) {
			r.With(appmiddleware.RequireRole("super_admin", "palace_editor")).Get("/", announcementAdminHandler.List)
			r.With(appmiddleware.RequireRole("super_admin", "palace_editor")).Post("/", announcementAdminHandler.Create)
			r.With(appmiddleware.RequireRole("super_admin", "palace_editor")).Put("/{id}", announcementAdminHandler.Update)
			r.With(appmiddleware.RequireRole("super_admin", "palace_editor")).Post("/{id}/submit", announcementAdminHandler.Submit)
			r.With(publisherRoles).Post("/{id}/approve", announcementAdminHandler.Approve)
			r.With(publisherRoles).Post("/{id}/publish", announcementAdminHandler.Publish)
			r.With(publisherRoles).Post("/{id}/archive", announcementAdminHandler.Archive)
		})

		// Decrees: the full versioned workflow. Same split as announcements —
		// palace_editor drafts and submits; only palace_publisher/super_admin
		// can approve, publish, or archive. Corrections re-enter as new drafts.
		r.Route("/decrees", func(r chi.Router) {
			editRoles := appmiddleware.RequireRole("super_admin", "palace_editor")
			r.With(editRoles).Get("/", decreeAdminHandler.List)
			r.With(editRoles).Get("/{id}", decreeAdminHandler.Get)
			r.With(editRoles).Post("/", decreeAdminHandler.Create)
			r.With(editRoles).Post("/{id}/submit", decreeAdminHandler.Submit)
			r.With(editRoles).Post("/{id}/correct", decreeAdminHandler.Correct)
			r.With(publisherRoles).Post("/{id}/approve", decreeAdminHandler.Approve)
			r.With(publisherRoles).Post("/{id}/publish", decreeAdminHandler.Publish)
			r.With(publisherRoles).Post("/{id}/archive", decreeAdminHandler.Archive)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("ochidoma-platform listening on :%s (env=%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("server stopped cleanly")
}

// secureHeaders sets baseline security headers on every response.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}
