package ghxd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const CredentialBundleVersion = 1

// CredentialBundle is ghxd's transport/checkpoint format for GitHub device-flow
// credentials. ghxd never persists this value; callers decide where durable
// copies live (for example, a GitHub Actions repository secret).
type CredentialBundle struct {
	Version               int       `json:"version"`
	ClientID              string    `json:"client_id"`
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
	TokenType             string    `json:"token_type,omitempty"`
	Scope                 string    `json:"scope,omitempty"`
}

// DeviceTokenResponse mirrors the stateless github-device-auth token JSON.
type DeviceTokenResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	TokenType             string `json:"token_type,omitempty"`
	Scope                 string `json:"scope,omitempty"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
}

func NewCredentialBundle(clientID string, token DeviceTokenResponse, now time.Time) (CredentialBundle, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return CredentialBundle{}, errors.New("client_id is required")
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		return CredentialBundle{}, errors.New("token response requires access_token and refresh_token")
	}
	if token.ExpiresIn <= 0 || token.RefreshTokenExpiresIn <= 0 {
		return CredentialBundle{}, errors.New("token response requires positive access and refresh expiries")
	}
	now = now.UTC()
	return CredentialBundle{
		Version:               CredentialBundleVersion,
		ClientID:              clientID,
		AccessToken:           token.AccessToken,
		RefreshToken:          token.RefreshToken,
		AccessTokenExpiresAt:  now.Add(time.Duration(token.ExpiresIn) * time.Second),
		RefreshTokenExpiresAt: now.Add(time.Duration(token.RefreshTokenExpiresIn) * time.Second),
		TokenType:             token.TokenType,
		Scope:                 token.Scope,
	}, nil
}

func (b CredentialBundle) Validate() error {
	if b.Version != CredentialBundleVersion {
		return fmt.Errorf("unsupported credential bundle version %d", b.Version)
	}
	if strings.TrimSpace(b.ClientID) == "" {
		return errors.New("credential bundle client_id is required")
	}
	if b.AccessToken == "" || b.RefreshToken == "" {
		return errors.New("credential bundle requires access_token and refresh_token")
	}
	if b.AccessTokenExpiresAt.IsZero() || b.RefreshTokenExpiresAt.IsZero() {
		return errors.New("credential bundle requires both expiry boundaries")
	}
	return nil
}

func (b CredentialBundle) AccessFresh(now time.Time, safetyMargin time.Duration) bool {
	if b.Validate() != nil {
		return false
	}
	if safetyMargin < 0 {
		safetyMargin = 0
	}
	return now.UTC().Add(safetyMargin).Before(b.AccessTokenExpiresAt.UTC())
}

func ParseCredentialBundle(data []byte) (CredentialBundle, error) {
	var bundle CredentialBundle
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bundle); err != nil {
		return CredentialBundle{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return CredentialBundle{}, errors.New("credential bundle contains trailing JSON")
		}
		return CredentialBundle{}, fmt.Errorf("credential bundle trailing data: %w", err)
	}
	if err := bundle.Validate(); err != nil {
		return CredentialBundle{}, err
	}
	return bundle, nil
}

func MarshalCredentialBundle(bundle CredentialBundle) ([]byte, error) {
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(bundle)
}
