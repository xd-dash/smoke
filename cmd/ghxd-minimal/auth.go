package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
			if token.AccessToken == "" {
				return errors.New("GitHub returned neither a token nor a device-flow status")
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

func writeEncryptedToken(ctx context.Context, keyFile, output string, token accessToken) error {
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	publicKeyCmd := exec.CommandContext(ctx, "age-keygen", "-y", keyFile)
	publicKeyCmd.Env = os.Environ()
	recipientOutput, err := publicKeyCmd.Output()
	if err != nil {
		return fmt.Errorf("derive public age recipient: %w", err)
	}
	recipient := strings.TrimSpace(string(recipientOutput))
	if !strings.HasPrefix(recipient, "age1") {
		return errors.New("invalid age recipient")
	}
	cmd := exec.CommandContext(ctx, "sops", "encrypt", "--age", recipient,
		"--input-type", "json", "--output-type", "json", "/dev/stdin")
	cmd.Stdin = bytes.NewReader(data)
	encrypted, err := cmd.Output()
	if err != nil {
		return errors.New("SOPS encryption failed; plaintext was not persisted")
	}
	if !bytes.Contains(encrypted, []byte(`"sops"`)) || bytes.Contains(encrypted, []byte(token.AccessToken)) {
		return errors.New("SOPS output is not valid encrypted token material")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".ghxd-encrypted-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(encrypted); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		return errors.New("encrypted credential was created concurrently; refusing to overwrite")
	}
	return os.Rename(f.Name(), output)
}
