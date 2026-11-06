// Package secrets owns process-local SOPS and age operations, independent of
// the optional Smoke command registry. It deliberately avoids SOPS's CLI.
package secrets

import (
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "time"

 filippoage "filippo.io/age"
 "github.com/getsops/sops/v3"
 "github.com/getsops/sops/v3/aes"
 sopsage "github.com/getsops/sops/v3/age"
 "github.com/getsops/sops/v3/config"
 "github.com/getsops/sops/v3/decrypt"
 jsonstore "github.com/getsops/sops/v3/stores/json"
)

const documentVersion = "3.13.3"

// IdentityFile reads a dedicated age identity only from the configured file.
// Never pass its bytes through environment variables or Nix derivations.
func IdentityFile(path string) (*filippoage.X25519Identity, error) {
 if path == "" { return nil, errors.New("age identity path required") }
 info, err := os.Lstat(path)
 if err != nil { return nil, err }
 if !info.Mode().IsRegular() || info.Mode().Perm() & 0077 != 0 {
  return nil, errors.New("age identity must be a private regular file")
 }
 raw, err := os.ReadFile(path)
 if err != nil { return nil, err }
 ids, err := filippoage.ParseIdentities(bytes.NewReader(raw))
 if err != nil || len(ids) != 1 { return nil, errors.New("expected one valid age identity") }
 id, ok := ids[0].(*filippoage.X25519Identity)
 if !ok { return nil, errors.New("expected X25519 age identity") }
 return id, nil
}

// EncryptJSON writes a valid SOPS JSON document encrypted to the public
// recipient belonging to identityPath. SOPS's encryption API is not stable:
// upgrades must be tested for compatibility with the pinned SOPS release.
func EncryptJSON(plaintext []byte, identityPath string) ([]byte, error) {
 id, err := IdentityFile(identityPath)
 if err != nil { return nil, err }
 store := jsonstore.NewStore(&config.JSONStoreConfig{})
 branches, err := store.LoadPlainFile(plaintext)
 if err != nil { return nil, errors.New("invalid secret JSON document") }
 if len(branches) != 1 || store.HasSopsTopLevelKey(branches[0]) {
  return nil, errors.New("invalid or already encrypted SOPS input")
 }
 mk, err := sopsage.MasterKeyFromRecipient(id.Recipient().String())
 if err != nil { return nil, err }
 tree := sops.Tree{
  Branches: branches,
  Metadata: sops.Metadata{
   KeyGroups: []sops.KeyGroup{{mk}},
   Version: documentVersion,
  },
 }
 key, errs := tree.GenerateDataKey()
 if len(errs) != 0 { return nil, errors.New("unable to wrap SOPS document key") }
 cipher := aes.NewCipher()
 mac, err := tree.Encrypt(key, cipher)
 if err != nil { return nil, errors.New("SOPS document encryption failed") }
 tree.Metadata.LastModified = time.Now().UTC()
 tree.Metadata.MessageAuthenticationCode, err = cipher.Encrypt(mac, key, tree.Metadata.LastModified.Format(time.RFC3339))
 if err != nil { return nil, errors.New("SOPS MAC encryption failed") }
 encrypted, err := store.EmitEncryptedFile(tree)
 if err != nil { return nil, errors.New("SOPS document serialization failed") }
 if bytes.Contains(encrypted, plaintext) { return nil, errors.New("SOPS output contains plaintext") }
 return encrypted, nil
}

// DecryptJSON delegates verification of SOPS metadata and its MAC to SOPS's
// stable decrypt API. The caller is responsible for plaintext lifetime.
func DecryptJSON(encrypted []byte, identityPath string) ([]byte, error) {
 if _, err := IdentityFile(identityPath); err != nil { return nil, err }
 prev, existed := os.LookupEnv("SOPS_AGE_KEY_FILE")
 // SOPS's stable helper uses process environment for identity discovery.
 // Use only in a single-purpose process; concurrent callers must not mutate
 // SOPS_AGE_KEY_FILE. We normally set this in the Quadlet before startup.
 if existed && prev != identityPath {
  return nil, errors.New("SOPS identity path differs from container configuration")
 }
 if !existed {
  return nil, errors.New("SOPS_AGE_KEY_FILE must be set by the runtime")
 }
 out, err := decrypt.Data(encrypted, "json")
 if err != nil { return nil, errors.New("SOPS decryption or MAC verification failed") }
 return out, nil
}

// SaveExclusive publishes only ciphertext and fails closed if the file exists.
// O_EXCL prevents two device-flow approvals racing to replace an old token.
func SaveExclusive(path string, ciphertext []byte) error {
 dir := filepath.Dir(path)
 if err := os.MkdirAll(dir, 0700); err != nil { return err }
 tmp, err := os.CreateTemp(dir, ".sops-ciphertext-*")
 if err != nil { return err }
 defer os.Remove(tmp.Name())
 if err := tmp.Chmod(0600); err != nil { _ = tmp.Close(); return err }
 if _, err := tmp.Write(ciphertext); err != nil { _ = tmp.Close(); return err }
 if err := tmp.Sync(); err != nil { _ = tmp.Close(); return err }
 if err := tmp.Close(); err != nil { return err }
 // POSIX link is atomic and fails with EEXIST. Rename alone could silently
 // overwrite credentials created by a concurrent device-flow approval.
 if err := os.Link(tmp.Name(), path); err != nil { return err }
 if d, err := os.Open(dir); err == nil { _ = d.Sync(); _ = d.Close() }
 return nil
}

// DigestCiphertext is safe for qualification logs. It does not inspect plaintext.
func DigestCiphertext(data []byte) string {
 sum := sha256.Sum256(data)
 return hex.EncodeToString(sum[:])
}

func ValidateSecretPath(path string) error {
 if strings.HasPrefix(filepath.Clean(path), "/nix/store/") {
  return fmt.Errorf("secrets cannot be written in the immutable Nix store")
 }
 return nil
}
