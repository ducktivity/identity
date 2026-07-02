// Package auth implements passwordless email login: generating and verifying the one-time codes, normalizing emails, and delivering the codes by email.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math/big"
	"net/mail"
	"strings"
	"time"
)

// Login-code policy.
const (
	CodeLength     = 6
	CodeTTL        = 10 * time.Minute
	ResendCooldown = 60 * time.Second
	MaxAttempts    = 5
)

// pepper peppers login-code hashes so a database leak cannot reveal in-flight codes and they cannot be brute-forced offline without the secret. Set once at startup via Init from AUTH_CODE_PEPPER.
var pepper []byte

// Init sets the login-code pepper. Call once at startup.
func Init(codePepper string) {
	pepper = []byte(codePepper)
}

// GenerateCode returns a cryptographically random 6-digit numeric code, zero-padded.
func GenerateCode() (string, error) {
	const digits = "0123456789"
	buf := make([]byte, CodeLength)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		buf[i] = digits[n.Int64()]
	}
	return string(buf), nil
}

// HashCode returns the hex SHA-256 of the code peppered with the server secret.
func HashCode(code string) string {
	h := sha256.New()
	h.Write([]byte(code))
	h.Write(pepper)
	return hex.EncodeToString(h.Sum(nil))
}

// Matches reports whether a candidate hashes to the stored hash, in constant time so verification latency does not leak how many characters were correct.
func Matches(candidate, storedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashCode(candidate)), []byte(storedHash)) == 1
}

// NormalizeEmail validates and lowercases an email so the UNIQUE constraint treats "Me@x.com" and "me@x.com" as one account. Returns ok=false when invalid.
func NormalizeEmail(raw string) (string, bool) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return strings.ToLower(addr.Address), true
}
