package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/httplog/v2"

	"github.com/ducktivity/identity/backend/database"
	"github.com/ducktivity/identity/backend/token"
)

// Healthz is the liveness probe: 200 whenever the process is running. It checks no
// dependencies so it stays cheap and never fails for a transient database blip.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Readyz is the readiness probe: 200 only when the database is reachable, 503
// otherwise. The deploy reconcile gates cutover on this probe.
func Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := database.Ping(ctx); err != nil {
		// Attach the cause to the request summary; do NOT report to Sentry — a DB
		// outage would otherwise flood it with one event per probe.
		httplog.LogEntrySetFields(r.Context(), map[string]any{"error": err.Error()})
		http.Error(w, `{"error":"database unreachable"}`, http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// JWKS serves the identity service's public key set. App backends fetch this to
// verify tokens. No auth — public keys only.
func JWKS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(token.PublicJWKS())
}
