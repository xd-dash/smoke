package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Axiom ingests callback messages into an Axiom edge deployment dataset.
// API tokens are resolved from the environment and are never embedded in the
// callback URL or callback source metadata.
type Axiom struct {
	Dataset         string
	Domain          string
	TokenEnv        string
	TimestampField  string
	TimestampFormat string
	EventLabels     string
	Client          *http.Client
}

func (a Axiom) Name() string { return "axiom" }

func (a Axiom) Handle(ctx context.Context, message Message) error {
	if a.Dataset == "" {
		return fmt.Errorf("dataset is required")
	}
	domain := strings.TrimSpace(a.Domain)
	if domain == "" {
		domain = strings.TrimSpace(os.Getenv("AXIOM_DOMAIN"))
	}
	if domain == "" {
		return fmt.Errorf("Axiom edge domain is required")
	}
	tokenEnv := strings.TrimSpace(a.TokenEnv)
	if tokenEnv == "" {
		tokenEnv = "AXIOM_TOKEN"
	}
	token := os.Getenv(tokenEnv)
	if token == "" {
		return fmt.Errorf("Axiom token environment %s is empty", tokenEnv)
	}

	u := url.URL{Scheme: "https", Host: domain, Path: "/v1/ingest/" + url.PathEscape(a.Dataset)}
	q := u.Query()
	if a.TimestampField != "" {
		q.Set("timestamp-field", a.TimestampField)
	}
	if a.TimestampFormat != "" {
		q.Set("timestamp-format", a.TimestampFormat)
	}
	u.RawQuery = q.Encode()

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal Axiom event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build Axiom request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if a.EventLabels != "" {
		req.Header.Set("X-Axiom-Event-Labels", a.EventLabels)
	}

	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Axiom ingest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Axiom ingest returned %s", resp.Status)
	}
	return nil
}

func parseAxiomURL(u *url.URL) (Callback, error) {
	dataset := strings.TrimSpace(u.Host)
	if dataset == "" {
		dataset = strings.Trim(strings.TrimSpace(u.Path), "/")
	}
	if dataset == "" {
		return nil, fmt.Errorf("axiom callback requires dataset id")
	}
	q := u.Query()
	return Axiom{
		Dataset:         dataset,
		Domain:          q.Get("domain"),
		TokenEnv:        q.Get("token-env"),
		TimestampField:  q.Get("timestamp-field"),
		TimestampFormat: q.Get("timestamp-format"),
		EventLabels:     q.Get("event-labels"),
	}, nil
}
