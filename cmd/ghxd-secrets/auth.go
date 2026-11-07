package ghxdsecrets

import (
		"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
			"strings"
	"time"

 "github.com/xd-dash/smoke/secrets"
)

type deviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Error           string `json:"error"`
}

type accessToken struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	TokenType             string `json:"token_type,omitempty"`
	Scope                 string `json:"scope,omitempty"`
	ExpiresIn             int    `json:"expires_in,omitempty"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in,omitempty"`
	Error                 string `json:"error,omitempty"`
}

var githubClient = &http.Client{Timeout: 20 * time.Second}

func requestOAuth(ctx context.Context, endpoint string, form url.Values, value any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := githubClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub device authorization returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(value)
}

func bootstrapGitHubToken(ctx context.Context) error {
	clientID := strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID"))
	if clientID == "" {
		return errors.New("GITHUB_CLIENT_ID is required for the interactive GitHub device flow")
	}
	keyFile := strings.TrimSpace(os.Getenv("SOPS_AGE_KEY_FILE"))
	if keyFile == "" {
		return errors.New("SOPS_AGE_KEY_FILE is required")
	}
	output := strings.TrimSpace(os.Getenv("GHXD_SOPS_FILE"))
	if output == "" {
		output = "/workspaces/.private/sops/ghxd.enc.json"
	}
	if _, err := os.Stat(keyFile); err != nil {
		return fmt.Errorf("age identity not mounted: %w", err)
	}
	if _, err := os.Stat(output); err == nil {
		return errors.New("encrypted GitHub credential already exists; refusing to replace it without an explicit rotation procedure")
	} else if !os.IsNotExist(err) {
		return err
	}
	var device deviceCode
	if err := requestOAuth(ctx, "https://github.com/login/device/code", url.Values{
		"client_id": {clientID},
		"scope": {"offline_access"},
	}, &device); err != nil {
		return fmt.Errorf("request device code: %w", err)
	}
	if device.Error != "" || device.DeviceCode == "" || device.UserCode == "" || device.ExpiresIn <= 0 {
		return fmt.Errorf("GitHub device flow unavailable: %s", device.Error)
	}
	if device.Interval < 5 {
		device.Interval = 5
	}
	fmt.Printf("Authorize GitHub at %s using code %s\n", device.VerificationURI, device.UserCode)
	fmt.Println("A token will be encrypted locally; no token material is printed.")
	deadlineCtx, cancel := context.WithTimeout(ctx, time.Duration(device.ExpiresIn)*time.Second)
	defer cancel()
	for {
		timer := time.NewTimer(time.Duration(device.Interval) * time.Second)
		select {
		case <-deadlineCtx.Done():
			timer.Stop()
			return fmt.Errorf("device authorization expired or cancelled: %w", deadlineCtx.Err())
		case <-timer.C:
		}
		var token accessToken
		err := requestOAuth(deadlineCtx, "https://github.com/login/oauth/access_token", url.Values{
			"client_id":   {clientID},
			"device_code": {device.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}, &token)
		if err != nil {
			return fmt.Errorf("poll GitHub device flow: %w", err)
		}
		switch token.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			device.Interval += 5
			continue
		case "":
			if token.AccessToken == "" || token.RefreshToken == "" {
                return errors.New("GitHub did not return both access and refresh tokens; enable expiring user tokens before authorization")
            }
			if err := writeEncryptedToken(ctx, keyFile, output, token); err != nil {
				return err
			}
			fmt.Printf("GitHub credential encrypted at %s\n", output)
			return nil
		default:
			return fmt.Errorf("GitHub authorization failed: %s", token.Error)
		}
	}
}

func writeEncryptedToken(_ context.Context, keyFile, output string, token accessToken) error {
 if err := secrets.ValidateSecretPath(output); err != nil { return err }
 data, err := json.Marshal(token)
 if err != nil { return err }
 encrypted, err := secrets.EncryptJSON(data, keyFile)
 if err != nil { return errors.New("native SOPS encryption failed") }
 return secrets.SaveExclusive(output, encrypted)
}

func checkEncryptedCredential(_ context.Context) error {
	output := strings.TrimSpace(os.Getenv("GHXD_SOPS_FILE"))
	if output == "" {
		output = "/workspaces/.private/sops/ghxd.enc.json"
	}
	if strings.TrimSpace(os.Getenv("SOPS_AGE_KEY_FILE")) == "" {
		return errors.New("SOPS_AGE_KEY_FILE is required")
	}
	keyFile := strings.TrimSpace(os.Getenv("SOPS_AGE_KEY_FILE"))
 encrypted, err := os.ReadFile(output)
 if err != nil { return errors.New("encrypted GitHub credential unavailable") }
 data, err := secrets.DecryptJSON(encrypted, keyFile)
 if err != nil { return errors.New("encrypted GitHub credential cannot be verified") }
	var token accessToken
	if err := json.Unmarshal(data, &token); err != nil {
		return errors.New("decrypted GitHub credential is not valid JSON")
	}
	if token.AccessToken == "" {
		return errors.New("decrypted GitHub credential has no access token")
	}
	fmt.Printf("GHXD_CREDENTIAL=READABLE refresh_token_present=%t scope=%s\n", token.RefreshToken != "", token.Scope)
	return nil
}
