// Command identity is the Ducktivity suite's central account + token service.
//
// It is deliberately the ONLY service that signs session tokens. Apps (Drinkwater, Wallet, …) never sign; they verify the tokens this service mints by fetching the public key from the JWKS endpoint. That asymmetry is the whole point of a shared identity: one issuer, many verifiers, no signing secret spread across apps. The suite-wide entitlement (one payment unlocks every app) is also stamped here, so apps read access from the token without ever talking to Stripe.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
	"github.com/joho/godotenv"

	"github.com/ducktivity/identity/backend/auth"
	"github.com/ducktivity/identity/backend/database"
	"github.com/ducktivity/identity/backend/handlers"
	"github.com/ducktivity/identity/backend/token"
)

// Build information, injected at release time via the linker:
//
//	go build -ldflags "-X main.version=1.2.3 -X main.commit=$(git rev-parse HEAD)"
//
// When built without ldflags (local `go run`), resolveBuildInfo() backfills these from the embedded VCS stamp so logs and the Sentry release still carry a commit.
var (
	version   = ""
	commit    = ""
	buildTime = ""
)

// config holds runtime configuration sourced from environment variables.
type config struct {
	env         string     // ENV: prod | staging | development; tags every log + Sentry event
	port        string     // PORT: TCP port to listen on
	logLevel    slog.Level // LOG_LEVEL: debug | info | warn | error
	logJSON     bool       // LOG_FORMAT: "json" (prod) or "text" (local dev pretty output)
	sentryDSN   string     // SENTRY_DSN: empty disables Sentry (no-op) for local dev
	issuer      string     // ISSUER: the "iss" claim + base URL, e.g. https://id.ducktvt.com
	signingKey  string     // AUTH_SIGNING_KEY: base64 Ed25519 seed; empty generates an ephemeral dev key
	codePepper  string     // AUTH_CODE_PEPPER: peppers login-code hashes
	resendKey   string     // RESEND_API_KEY: empty logs login codes instead of emailing (dev)
	emailFrom   string     // AUTH_EMAIL_FROM: From header for login emails
	stripeWHSec string     // STRIPE_WEBHOOK_SECRET: verifies billing webhook signatures (stub today)
}

func loadConfig() config {
	env := getenv("ENV", "development")
	// Local dev defaults to pretty text; staging/prod default to JSON so the log aggregator can parse it. An explicit LOG_FORMAT still wins in any environment.
	defaultLogFormat := "json"
	if env == "development" {
		defaultLogFormat = "text"
	}
	return config{
		env:         env,
		port:        getenv("PORT", "8000"),
		logLevel:    httplog.LevelByName(getenv("LOG_LEVEL", "info")),
		logJSON:     getenv("LOG_FORMAT", defaultLogFormat) == "json",
		sentryDSN:   os.Getenv("SENTRY_DSN"),
		issuer:      getenv("ISSUER", "http://localhost:8000"),
		signingKey:  os.Getenv("AUTH_SIGNING_KEY"),
		codePepper:  os.Getenv("AUTH_CODE_PEPPER"),
		resendKey:   os.Getenv("RESEND_API_KEY"),
		emailFrom:   getenv("AUTH_EMAIL_FROM", "Ducktivity <onboarding@resend.dev>"),
		stripeWHSec: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
}

func main() {
	resolveBuildInfo()
	_ = godotenv.Load()
	cfg := loadConfig()

	// One logger powers everything: httplog emits a single concise summary line per request, and slog.SetDefault routes startup/shutdown/DB logs through the same pipeline. Health probes are quieted so they don't drown the logs (or the BetterStack bill).
	logger := httplog.NewLogger("identity-backend", httplog.Options{
		JSON:             cfg.logJSON,
		LogLevel:         cfg.logLevel,
		Concise:          true,
		MessageFieldName: "msg",
		QuietDownRoutes:  []string{"/healthz", "/readyz"},
		QuietDownPeriod:  1 * time.Hour,
	})
	// Base attributes ride on every line so an aggregator can filter by env/version/commit/pid. Skipped in local text mode to keep each line short and readable.
	if cfg.logJSON {
		logger.Logger = logger.Logger.With(
			"env", cfg.env,
			"version", version,
			"commit", shortCommit(commit),
			"pid", os.Getpid(),
		)
	}
	slog.SetDefault(logger.Logger)

	// The code pepper hashes login codes. Fail fast outside dev rather than run with a guessable default.
	if cfg.codePepper == "" {
		if cfg.env != "development" {
			slog.Error("AUTH_CODE_PEPPER is required outside development")
			os.Exit(1)
		}
		slog.Warn("AUTH_CODE_PEPPER is empty; using an insecure development default")
		cfg.codePepper = "dev-insecure-pepper-do-not-use-in-prod"
	}
	auth.Init(cfg.codePepper)
	auth.InitEmail(cfg.resendKey, cfg.emailFrom)

	// Load (or, in dev, generate) the Ed25519 signing key that backs both token issuance and the published JWKS, and set the issuer claim.
	if err := token.Init(cfg.signingKey, cfg.issuer); err != nil {
		slog.Error("failed to load signing key", "error", err)
		os.Exit(1)
	}

	handlers.Init(cfg.stripeWHSec)

	database.Connect() // database pool against the shared Neon (identity reads/writes users, auth_codes, entitlements)

	// Sentry captures stack traces, groups errors, and alerts. With an empty DSN it is a no-op, so local dev needs no Sentry account.
	if cfg.sentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.sentryDSN,
			Environment:      cfg.env,
			Release:          version + "+" + shortCommit(commit),
			AttachStacktrace: true,
			EnableTracing:    false, // errors only; we run no tracing backend
		}); err != nil {
			slog.Error("sentry init failed", "error", err)
		} else {
			defer sentry.Flush(2 * time.Second)
		}
	}

	r := chi.NewRouter()

	// httplog wraps everything so it logs the final status; Sentry sits inside Recoverer with Repanic:true so a panic is captured with its stack trace and then re-panicked up to Recoverer, which turns it into a clean 500.
	//
	// We deliberately do NOT register middleware.RequestID ourselves: httplog's RequestLogger already chains it internally and logs the id as "requestID". Adding our own would assign a *different* id, so the id we echo to the caller wouldn't match the logged one.
	r.Use(httplog.RequestLogger(logger))
	// Echo that same request id onto the response so a caller (or the frontend that eventually surfaces it) can cross-reference a report against the exact log line. Runs after RequestLogger so it reads the id httplog already generated.
	r.Use(echoRequestID)
	r.Use(middleware.Recoverer)
	r.Use(sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle)

	// The browser talks to identity directly for login: app frontends POST to /v1/auth/request-code and /v1/auth/verify-code here (identity is the sole issuer), so those calls are cross-origin and need CORS. We allow any origin at the app and enforce which frontends may actually reach identity at the Cloudflare edge instead — that keeps origin policy in one place and lets a new app come online without a code change. Wildcard is safe here because identity authenticates with bearer tokens in the Authorization header, not cookies, so there are no credentialed requests to widen (browsers forbid "*" + credentials anyway).
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		// Surface the per-request id (written by echoRequestID above) to the browser. Browsers hide non-simple response headers from JS unless they're explicitly exposed; without this the frontend can't read the id to show users for support reports.
		ExposedHeaders: []string{"X-Request-Id"},
	}))

	// Unversioned infrastructure endpoints: liveness, readiness, and the public key set app backends fetch to verify tokens (no auth — public keys only).
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz)
	r.Get("/.well-known/jwks.json", handlers.JWKS)

	// Versioned public API. Grouping under /v1 keeps the version boundary a single explicit node and gives API-wide middleware (rate limits, etc.) one home, separate from the infra routes above.
	r.Route("/v1", func(r chi.Router) {
		// Passwordless login: request a 6-digit code by email, then exchange it for a token whose entitlement reflects the account's current suite-wide access.
		r.Route("/auth", func(r chi.Router) {
			r.Post("/request-code", handlers.AuthRequestCode)
			r.Post("/verify-code", handlers.AuthVerifyCode)
		})

		// Billing: Stripe posts subscription lifecycle events here; we resolve them to the single suite-wide entitlement. Signature-verified (stub today).
		r.Post("/billing/webhook", handlers.BillingWebhook)

		// Dev-only: flip a user's entitlement without Stripe, to exercise the "one payment unlocks all apps" path end-to-end before billing is wired.
		if cfg.env == "development" {
			r.Post("/dev/grant", handlers.DevGrant)
		}
	})

	srv := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Drain in-flight requests on SIGINT/SIGTERM so a deploy cutover doesn't drop them — and so the deferred sentry.Flush actually runs before exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("identity service starting", "port", cfg.port, "issuer", cfg.issuer, "env", cfg.env, "version", version, "commit", shortCommit(commit))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			stop() // unblock main so the process exits via the shutdown path below
		}
	}()

	<-ctx.Done()
	stop()
	slog.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
	database.DB.Close()
	slog.Info("server stopped")
}

// resolveBuildInfo backfills empty ldflags-injected build vars from the Go toolchain's embedded VCS stamp, so a binary built with a plain `go build` still reports its commit.
func resolveBuildInfo() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "" {
		version = bi.Main.Version // e.g. "(devel)" for local builds
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "" {
				commit = s.Value
			}
		case "vcs.time":
			if buildTime == "" {
				buildTime = s.Value
			}
		}
	}
	if version == "" {
		version = "dev"
	}
	if commit == "" {
		commit = "unknown"
	}
}

// echoRequestID copies the chi request id into the X-Request-Id response header. It must run after httplog.RequestLogger (which internally runs middleware.RequestID and logs the id as "requestID") so the header carries the exact same id. The header is set before the handler writes the body, so it survives even on error responses.
func echoRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqID := middleware.GetReqID(r.Context()); reqID != "" {
			w.Header().Set("X-Request-Id", reqID)
		}
		next.ServeHTTP(w, r)
	})
}

// shortCommit trims a git SHA to its first 12 characters for readable logs.
func shortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
