// Package api holds the request and response DTOs at the HTTP boundary. Naming each wire shape (rather than using inline anonymous structs in the handlers) gives every endpoint a reusable type and one place to read the contract.
package api

import "github.com/google/uuid"

// AuthRequestInput is the body of POST /v1/auth/request.
type AuthRequestInput struct {
	Email string `json:"email" validate:"required"`
}

// AuthVerifyInput is the body of POST /v1/auth/verify.
type AuthVerifyInput struct {
	Email string `json:"email" validate:"required"`
	Code  string `json:"code" validate:"required"`
}

// DevGrantInput is the body of POST /v1/dev/grant (development only).
type DevGrantInput struct {
	Email string `json:"email" validate:"required"`
	Plan  string `json:"plan"` // "pro" | "free"
	Days  int    `json:"days"` // optional validity window for pro
}

// User is the account shape returned to clients.
type User struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

// VerifyResponse is returned by POST /v1/auth/verify: the session token plus the account it belongs to.
type VerifyResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// GrantResponse is returned by POST /v1/dev/grant.
type GrantResponse struct {
	UserID uuid.UUID `json:"user_id"`
	Plan   string    `json:"plan"`
}

// MessageResponse is a generic single-message body (e.g. the deliberately vague acknowledgement returned by POST /v1/auth/request).
type MessageResponse struct {
	Message string `json:"message"`
}

// ErrorResponse is the single error shape every endpoint returns on failure.
type ErrorResponse struct {
	Error string `json:"error"`
}
