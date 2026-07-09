package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ducktivity/identity/backend/api"
	"github.com/ducktivity/identity/backend/auth"
	"github.com/ducktivity/identity/backend/store"
	"github.com/ducktivity/identity/backend/token"
)

// AuthRequestCode sends a 6-digit login code, creating the account if needed. It always returns a generic acknowledgement so it cannot probe which emails have accounts.
func AuthRequestCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in api.AuthRequestCodeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	addr, ok := auth.NormalizeEmail(in.Email)
	if !ok {
		writeErr(w, http.StatusBadRequest, "A valid email is required")
		return
	}

	u, err := store.UpsertUserByEmail(ctx, addr)
	if err != nil {
		serverError(w, r, err, "Could not start login")
		return
	}

	// Per-account cooldown: refuse if the most recent still-valid code is too fresh.
	if _, createdAt, err := store.GetLatestActiveAuthCode(ctx, u.ID); err == nil {
		if time.Since(createdAt) < auth.ResendCooldown {
			writeErr(w, http.StatusTooManyRequests, "Please wait a moment before requesting another code")
			return
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, r, err, "Could not start login")
		return
	}

	code, err := auth.GenerateCode()
	if err != nil {
		serverError(w, r, err, "Could not start login")
		return
	}
	if err := store.CreateAuthCode(ctx, u.ID, auth.HashCode(code), time.Now().Add(auth.CodeTTL)); err != nil {
		serverError(w, r, err, "Could not start login")
		return
	}
	if err := auth.SendLoginCode(ctx, addr, code); err != nil {
		serverError(w, r, err, "Could not send login email")
		return
	}
	writeJSON(w, http.StatusOK, api.MessageResponse{Message: "If that email is valid, a code is on its way."})
}

// AuthVerifyCode exchanges a valid email + code for a session token. The token carries the account's current suite-wide entitlement, so every app learns paid access from the token alone.
func AuthVerifyCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in api.AuthVerifyCodeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	addr, ok := auth.NormalizeEmail(in.Email)
	if !ok {
		writeErr(w, http.StatusBadRequest, "A valid email is required")
		return
	}

	// A missing user is reported with the same generic 401 as a wrong code so the endpoint can't enumerate accounts.
	u, err := store.GetUserByEmail(ctx, addr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusUnauthorized, "Invalid or expired code")
			return
		}
		serverError(w, r, err, "Could not verify code")
		return
	}

	code, _, err := store.GetLatestActiveAuthCode(ctx, u.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusUnauthorized, "Invalid or expired code")
			return
		}
		serverError(w, r, err, "Could not verify code")
		return
	}
	if code.Attempts >= auth.MaxAttempts {
		writeErr(w, http.StatusUnauthorized, "Invalid or expired code")
		return
	}
	if !auth.Matches(in.Code, code.CodeHash) {
		if err := store.IncrementAuthCodeAttempts(ctx, code.ID); err != nil {
			serverError(w, r, err, "Could not verify code")
			return
		}
		writeErr(w, http.StatusUnauthorized, "Invalid or expired code")
		return
	}
	if err := store.ConsumeAuthCode(ctx, code.ID); err != nil {
		serverError(w, r, err, "Could not verify code")
		return
	}

	ent, err := store.GetEntitlement(ctx, u.ID)
	if err != nil {
		serverError(w, r, err, "Could not verify code")
		return
	}

	tok, err := token.Issue(u.ID, u.Email, ent)
	if err != nil {
		serverError(w, r, err, "Could not verify code")
		return
	}
	writeJSON(w, http.StatusOK, api.VerifyResponse{
		Token: tok,
		User:  api.User{ID: u.ID, Email: u.Email},
	})
}
