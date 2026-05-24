// ward-duty-api 진입점.
// Design Ref: §9.1
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"ward-duty-api/internal/auth"
	"ward-duty-api/internal/db"
	"ward-duty-api/internal/levels"
	"ward-duty-api/internal/nightkeepers"
	"ward-duty-api/internal/notifications"
	"ward-duty-api/internal/nurses"
	"ward-duty-api/internal/schedules"
	"ward-duty-api/internal/settings"
	"ward-duty-api/internal/solver"
	"ward-duty-api/internal/swaps"
	"ward-duty-api/internal/wishes"
	"ward-duty-api/pkg/httpx"
)

func main() {
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps, err := db.Connect(ctx)
	if err != nil {
		slog.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer deps.Close()

	if os.Getenv("SKIP_MIGRATE") != "1" {
		if err := db.MigrateUp(ctx, deps); err != nil {
			slog.Warn("migrate skipped or failed", "err", err)
			// 운영에선 SQL을 수동으로 적용했을 수도. 치명적이지 않음.
		}
	}

	oauthCfg, err := auth.LoadOAuthConfig()
	if err != nil {
		if auth.IsDevMode() {
			slog.Warn("oauth config missing — dev-login만 사용 가능", "err", err)
			oauthCfg = &auth.OAuthConfig{
				ClientID:     "dev-client",
				ClientSecret: "dev-secret",
				RedirectURL:  "http://localhost:8080/api/auth/oauth/google/callback",
				StateSecret:  []byte("dev-state-secret-not-for-prod"),
				AdminEmail:   strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))),
			}
		} else {
			slog.Error("oauth config", "err", err)
			os.Exit(1)
		}
	}

	solverCli := solver.NewFromEnv()

	notifRepo := notifications.NewRepo(deps.PG)
	auth.SetPendingNotifyHook(func(ctx context.Context, email, name string) {
		heads, err := notifRepo.HeadNurseIDs(ctx)
		if err != nil {
			slog.Warn("pending notify: head ids", "err", err)
			return
		}
		display := name
		if display == "" {
			display = email
		}
		_ = notifRepo.InsertMany(ctx, heads, notifications.Create{
			Type:  notifications.TypeAccountPendingApproval,
			Title: "새 계정 연결 요청",
			Body:  display + " (" + email + ")",
			Link:  "/account-link",
			Meta:  map[string]any{"email": email},
		})
	})

	authH := auth.New(deps.PG, deps.Redis, oauthCfg)
	nursesH := nurses.New(deps.PG)
	levelsH := levels.New(deps.PG)
	settingsH := settings.New(deps.PG)
	wishesH := wishes.New(deps.PG)
	nkH := nightkeepers.New(deps.PG)
	schedulesH := schedules.New(deps.PG, solverCli)
	swapsH := swaps.New(deps.PG, solverCli)
	notifH := notifications.New(deps.PG)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(httpx.CORS()) // ALLOWED_ORIGINS env로 dev 허용
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Logger)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api", func(api chi.Router) {
		api.Route("/auth", func(a chi.Router) {
			a.Get("/oauth/google/start", authH.Start)
			a.Get("/oauth/google/callback", authH.Callback)
			a.Post("/refresh", authH.Refresh)
			// dev-login: ENV=development일 때만 노출
			if auth.IsDevMode() {
				a.Post("/dev-login", authH.DevLogin)
				slog.Warn("dev-login enabled — DO NOT use ENV=development in production")
			}
			// 인증 필요
			a.Group(func(p chi.Router) {
				p.Use(auth.RequireAuth)
				p.Get("/me", authH.Me)
				p.Post("/logout", authH.Logout)
			})
		})

		api.Group(func(p chi.Router) {
			p.Use(auth.RequireAuth)

			// 모두 접근 (RBAC은 핸들러 내부에서 본인 필터)
			p.Get("/nurses", nursesH.List)
			p.Get("/levels", levelsH.List)
			p.Get("/settings", settingsH.Get)

			// wishes: nurse 본인 / head_nurse 전체
			p.Get("/wishes", wishesH.List)
			p.Put("/wishes/{date}", wishesH.Upsert)
			p.Delete("/wishes/{date}", wishesH.Delete)

			// schedules
			p.Get("/schedules", schedulesH.GetByYM)
			p.Get("/schedules/{id}/cells", schedulesH.ListCells)

			// night-keepers
			p.Get("/night-keepers", nkH.List)

			// swaps
			p.Get("/swaps", swapsH.List)
			p.Post("/swaps", swapsH.Create)
			p.Patch("/swaps/{id}", swapsH.Patch)
			p.Post("/swaps/{id}/cancel", swapsH.Cancel)

			// notifications (본인만)
			p.Get("/notifications", notifH.List)
			p.Get("/notifications/unread-count", notifH.UnreadCount)
			p.Post("/notifications/{id}/read", notifH.Read)
			p.Post("/notifications/read-all", notifH.ReadAll)
			p.Delete("/notifications/{id}", notifH.Delete)

			// head_nurse only
			p.Group(func(hh chi.Router) {
				hh.Use(auth.RequireRole("head_nurse"))
				hh.Post("/nurses", nursesH.Create)
				hh.Patch("/nurses/{id}", nursesH.Update)
				// Stage 2 — pending OAuth 계정 매칭
				hh.Get("/pending-accounts", authH.ListPending)
				hh.Delete("/pending-accounts/{email}", authH.DismissPending)
				hh.Post("/nurses/{id}/link-account", authH.LinkAccount)

				hh.Post("/levels", levelsH.Create)
				hh.Patch("/levels/{code}", levelsH.Update)
				hh.Delete("/levels/{code}", levelsH.Delete)

				hh.Patch("/settings", settingsH.Update)

				hh.Post("/schedules", schedulesH.Create)
				hh.Patch("/schedules/{id}/cells/{cellId}", schedulesH.PatchCell)
				hh.Post("/schedules/{id}/confirm", schedulesH.Confirm)
				hh.Post("/schedules/{id}/reset", schedulesH.Reset) // C5 보조

				hh.Post("/night-keepers", nkH.Create)
				hh.Delete("/night-keepers/{id}", nkH.Delete)
			})
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			cancel()
		}
	}()

	<-sigCh
	slog.Info("shutting down...")
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	_ = srv.Shutdown(shutdownCtx)
}
