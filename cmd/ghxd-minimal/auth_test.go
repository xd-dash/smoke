package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptCredentialNeverWritesPlaintext(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "identity.txt")
	if err := os.WriteFile(key, []byte("synthetic-test-key"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		"age-keygen": "#!/bin/sh\nprintf 'age1testrecipient\\n'\n",
		"sops": "#!/bin/sh\ncat >/dev/null\nprintf '{\"ciphertext\":\"ENC[synthetic]\",\"sops\":{\"age\":[]}}\\n'\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output := filepath.Join(dir, "encrypted", "ghxd.enc.json")
	token := accessToken{AccessToken: "synthetic-access-token-do-not-log", RefreshToken: "synthetic-refresh-token"}
	if err := writeEncryptedToken(context.Background(), key, output, token); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), token.AccessToken) || strings.Contains(string(body), token.RefreshToken) {
		t.Fatal("unprotected token material persisted")
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("encrypted file mode %o, expected 0600", info.Mode().Perm())
	}
}
