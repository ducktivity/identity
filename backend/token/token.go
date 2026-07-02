// Package token owns the Ed25519 signing key and is the ONLY place in the suite that mints session tokens. It also publishes the matching public key as a JWKS so every app backend can verify tokens without holding any signing key.
package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ducktivity/platform-go/authtoken"
)

// tokenTTL is how long a session token stays valid. When the access/refresh split (docs/suite-architecture §4.3) lands, this becomes the short-lived access-token TTL and a refresh token is added so entitlement changes propagate within minutes instead of days.
const tokenTTL = 30 * 24 * time.Hour

// issuer is the "iss" claim stamped on every token; set from config via Init.
var issuer string

// signingKey holds the Ed25519 keypair used to sign tokens and publish JWKS.
var signingKey struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
	kid  string // key id, surfaced in token headers + JWKS so keys can rotate
}

// Init loads the signing key from a base64-encoded 32-byte seed (AUTH_SIGNING_KEY) and sets the issuer claim. An empty seed generates an ephemeral key — fine for local dev (every restart invalidates old tokens), never for prod or multiple replicas, which must share one key so any instance can verify another's tokens. Call once at startup.
func Init(seedB64, iss string) error {
	issuer = iss
	if seedB64 == "" {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		signingKey.priv, signingKey.pub, signingKey.kid = priv, pub, "dev-ephemeral"
		slog.Warn("AUTH_SIGNING_KEY empty; generated an ephemeral signing key (dev only; tokens reset on restart)")
		return nil
	}
	seed, err := base64.StdEncoding.DecodeString(seedB64)
	if err != nil {
		return err
	}
	if len(seed) != ed25519.SeedSize {
		return errors.New("AUTH_SIGNING_KEY must decode to a 32-byte Ed25519 seed")
	}
	priv := ed25519.NewKeyFromSeed(seed)
	signingKey.priv = priv
	signingKey.pub = priv.Public().(ed25519.PublicKey)
	// A stable kid derived from the public key lets verifiers cache across restarts and lets us add a second key during rotation without ambiguity.
	signingKey.kid = base64.RawURLEncoding.EncodeToString(signingKey.pub)[:12]
	return nil
}

// Issue mints a signed EdDSA session token. This is the only place in the entire suite that signs a token. The entitlement is stamped in so every app reads suite-wide access straight from the token.
func Issue(userID uuid.UUID, email string, ent authtoken.Entitlement) (string, error) {
	now := time.Now()
	claims := authtoken.Claims{
		Email:       email,
		Entitlement: ent,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = signingKey.kid // verifiers use this to pick the right JWKS key
	return tok.SignedString(signingKey.priv)
}

// PublicJWKS returns the Ed25519 public key as a JSON Web Key Set. Ed25519 keys use the OKP key type with curve "Ed25519" and the raw public key in "x" (base64url, unpadded). App backends fetch and cache this to verify tokens.
func PublicJWKS() map[string]any {
	return map[string]any{
		"keys": []map[string]any{{
			"kty": "OKP",
			"crv": "Ed25519",
			"x":   base64.RawURLEncoding.EncodeToString(signingKey.pub),
			"use": "sig",
			"alg": "EdDSA",
			"kid": signingKey.kid,
		}},
	}
}
