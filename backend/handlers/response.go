package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/httplog/v2"

	"github.com/ducktivity/identity/backend/api"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, api.ErrorResponse{Error: msg})
}

// serverError records a 5xx server fault: it reports the underlying error to Sentry (which captures the stack trace and groups the issue), links the resulting event id onto the single request-summary line httplog emits, and returns a generic JSON 500. The internal error is never leaked to the client.
func serverError(w http.ResponseWriter, r *http.Request, err error, clientMsg string) {
	fields := map[string]any{"error": err.Error()}
	// CaptureException is a no-op when Sentry is unconfigured (empty DSN in dev): the hub has no client and returns a nil event id.
	if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
		if id := hub.CaptureException(err); id != nil {
			fields["sentry_id"] = string(*id)
		}
	}
	httplog.LogEntrySetFields(r.Context(), fields)
	writeErr(w, http.StatusInternalServerError, clientMsg)
}
