// Package api holds the request and response DTOs at the HTTP boundary. Naming each wire shape (rather than using inline anonymous structs in the handlers) gives every endpoint a reusable type and one place to read the contract.
package api

import "github.com/google/uuid"

// AuthRequestCodeInput is the body of POST /v1/auth/request-code.
type AuthRequestCodeInput struct {
	Email string `json:"email" validate:"required"`
}

// AuthVerifyCodeInput is the body of POST /v1/auth/verify-code.
type AuthVerifyCodeInput struct {
	Email string `json:"email" validate:"required"`
	Code  string `json:"code" validate:"required"`
}

// DevGrantInput is the body of POST /v1/dev/grant (development only).
type DevGrantInput struct {
	Email string `json:"email" validate:"required"`
	Plan  string `json:"plan"` // "pro" | "free"
	Days  int    `json:"days"` // optional validity window for pro
}

// User is the account shape returned to clients. Both fields are always populated, so they are marked required for the generated schema (and thus the frontend types).
type User struct {
	ID    uuid.UUID `json:"id" validate:"required"`
	Email string    `json:"email" validate:"required"`
}

// VerifyResponse is returned by POST /v1/auth/verify-code: the session token plus the account it belongs to. Both fields are always present on a 200.
type VerifyResponse struct {
	Token string `json:"token" validate:"required"`
	User  User   `json:"user" validate:"required"`
}

// GrantResponse is returned by POST /v1/dev/grant.
type GrantResponse struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Plan   string    `json:"plan" validate:"required"`
}

// MessageResponse is a generic single-message body (e.g. the deliberately vague acknowledgement returned by POST /v1/auth/request-code).
type MessageResponse struct {
	Message string `json:"message" validate:"required"`
}

// ErrorResponse is the single error shape every endpoint returns on failure.
type ErrorResponse struct {
	Error string `json:"error" validate:"required"`
}
