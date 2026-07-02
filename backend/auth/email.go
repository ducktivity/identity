package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Email delivery via Resend.

const resendEndpoint = "https://api.resend.com/emails"

// sender is the process-wide Resend client, configured once via InitEmail.
var sender struct {
	apiKey string
	from   string
	client *http.Client
}

// InitEmail configures the Resend email sender. An empty apiKey selects dev mode: login codes are logged instead of emailed. Call once at startup.
func InitEmail(apiKey, from string) {
	sender.apiKey = apiKey
	sender.from = from
	sender.client = &http.Client{Timeout: 10 * time.Second}
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

// SendLoginCode emails a one-time login code. In dev mode (no API key) it logs the code and returns nil so the flow completes without an email provider.
func SendLoginCode(ctx context.Context, to, code string) error {
	if sender.apiKey == "" {
		slog.Info("login code (dev mode, email not sent)", "to", to, "code", code)
		return nil
	}
	subject := "Your Ducktivity login code"
	text := fmt.Sprintf("Your Ducktivity login code is %s. It expires in 10 minutes.", code)
	html := fmt.Sprintf(
		`<p>Your Ducktivity login code is:</p><p style="font-size:28px;font-weight:bold;letter-spacing:4px">%s</p><p>It expires in 10 minutes. If you didn't request this, you can ignore this email.</p>`,
		code,
	)
	body, err := json.Marshal(resendRequest{From: sender.from, To: []string{to}, Subject: subject, HTML: html, Text: text})
	if err != nil {
		return fmt.Errorf("marshal resend request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+sender.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := sender.client.Do(req)
	if err != nil {
		return fmt.Errorf("send via resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("resend returned %d: %s", resp.StatusCode, string(snippet))
	}
	return nil
}
