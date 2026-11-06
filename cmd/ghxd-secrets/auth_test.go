package ghxdsecrets

import (
 "context"
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "filippo.io/age"
 "github.com/xd-dash/smoke/secrets"
)

func TestNativeSOPSDeviceTokenPersistence(t *testing.T) {
 dir := t.TempDir()
 id, err := age.GenerateX25519Identity()
 if err != nil { t.Fatal(err) }
 key := filepath.Join(dir, "age-identity")
 if err := os.WriteFile(key, []byte(id.String()+"\n"), 0600); err != nil { t.Fatal(err) }
 t.Setenv("SOPS_AGE_KEY_FILE", key)
 output := filepath.Join(dir, "encrypted", "ghxd.enc.json")
 token := accessToken{AccessToken:"synthetic-access-do-not-log", RefreshToken:"synthetic-refresh-do-not-log"}
 if err := writeEncryptedToken(context.Background(), key, output, token); err != nil { t.Fatal(err) }
 body, err := os.ReadFile(output)
 if err != nil { t.Fatal(err) }
 for _, value := range []string{token.AccessToken, token.RefreshToken} {
  if strings.Contains(string(body), value) { t.Fatal("credential plaintext leaked into ciphertext") }
 }
 if !strings.Contains(string(body), "\"sops\"") { t.Fatal("not a SOPS document") }
 plaintext, err := secrets.DecryptJSON(body, key)
 if err != nil { t.Fatal(err) }
 var loaded accessToken
 if err := json.Unmarshal(plaintext, &loaded); err != nil { t.Fatal(err) }
 if loaded.AccessToken != token.AccessToken || loaded.RefreshToken != token.RefreshToken {
  t.Fatal("token roundtrip mismatch")
 }
 if err := writeEncryptedToken(context.Background(), key, output, token); err == nil {
  t.Fatal("overwrote an existing credential")
 }
}
