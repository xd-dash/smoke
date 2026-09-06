package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xd-dash/smoke/identity"
	"github.com/xd-dash/smoke/internal/filelock"
)

// Record is lightweight local supervision metadata for one running unattended
// Logmash process. It is not durable Logma graph state.
type Record struct {
	ID                string    `json:"id"`
	PID               int       `json:"pid"`
	Profile           string    `json:"profile"`
	Target            string    `json:"target"`
	Channels          []string  `json:"channels,omitempty"`
	Patterns          []string  `json:"patterns,omitempty"`
	Callbacks         []string  `json:"callbacks"`
	AuthProvider      string    `json:"auth_provider,omitempty"`
	CompositionDigest string    `json:"composition_digest,omitempty"`
	Environment       string    `json:"environment,omitempty"`
	WorkspaceDigest   string    `json:"workspace_digest,omitempty"`
	Workspace         string    `json:"workspace,omitempty"`
	Lease             string    `json:"lease,omitempty"`
	StartedAt         time.Time `json:"started_at"`
}

type Handle struct {
	path      string
	leasePath string
	lease     *filelock.Lock
}

func Begin(record Record) (*Handle, Record, error) {
	dir, err := directory()
	if err != nil {
		return nil, Record{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, Record{}, fmt.Errorf("create session directory: %w", err)
	}

	var id, leasePath string
	var lease *filelock.Lock
	for attempts := 0; attempts < 8; attempts++ {
		id, err = newID()
		if err != nil {
			return nil, Record{}, err
		}
		leasePath = filepath.Join(dir, id+".lock")
		var ok bool
		lease, ok, err = filelock.TryAcquire(leasePath, filelock.Exclusive)
		if err != nil {
			return nil, Record{}, fmt.Errorf("session lease: %w", err)
		}
		if ok {
			break
		}
		lease = nil
	}
	if lease == nil {
		return nil, Record{}, fmt.Errorf("could not allocate unique session lease")
	}

	record.ID = id
	record.PID = os.Getpid()
	record.StartedAt = time.Now().UTC()
	record.Callbacks = SanitizeCallbacks(record.Callbacks)
	record.CompositionDigest = identity.CompositionDigest()
	record.Environment = strings.TrimSpace(os.Getenv("SMOKE_ENV"))
	record.Workspace = strings.TrimSpace(os.Getenv("SMOKE_ENV_WORKSPACE"))
	record.WorkspaceDigest = identity.WorkspaceDigest()
	record.Lease = leasePath
	path := filepath.Join(dir, id+".json")
	if err := writeAtomic(path, record); err != nil {
		_ = lease.Close()
		_ = os.Remove(leasePath)
		return nil, Record{}, err
	}
	return &Handle{path: path, leasePath: leasePath, lease: lease}, record, nil
}

func (h *Handle) Close() error {
	if h == nil { return nil }
	var first error
	if h.path != "" {
		if err := os.Remove(h.path); err != nil && !os.IsNotExist(err) { first = err }
		h.path = ""
	}
	if h.lease != nil {
		if err := h.lease.Close(); err != nil && first == nil { first = err }
		h.lease = nil
	}
	if h.leasePath != "" {
		if err := os.Remove(h.leasePath); err != nil && !os.IsNotExist(err) && first == nil { first = err }
		h.leasePath = ""
	}
	return first
}

func List() ([]Record, error) {
	dir, err := directory(); if err != nil { return nil, err }
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) { return nil, nil }
	if err != nil { return nil, fmt.Errorf("read session directory: %w", err) }
	var records []Record
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") { continue }
		path := filepath.Join(dir, entry.Name())
		record, err := read(path); if err != nil { continue }
		live, err := leaseHeld(filepath.Join(dir, record.ID+".lock")); if err != nil { continue }
		if !live || !processAlive(record.PID) { cleanupStale(dir, record.ID); continue }
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt.Before(records[j].StartedAt) })
	return records, nil
}

func Stop(id string) (Record, error) {
	record, err := runningRecord(id); if err != nil { return Record{}, err }
	if err := stopProcess(record.PID); err != nil { return Record{}, fmt.Errorf("stop session %s: %w", id, err) }
	return record, nil
}

func runningRecord(id string) (Record, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, `/\\`) { return Record{}, fmt.Errorf("invalid session id") }
	dir, err := directory(); if err != nil { return Record{}, err }
	record, err := read(filepath.Join(dir, id+".json")); if err != nil { return Record{}, err }
	live, err := leaseHeld(filepath.Join(dir, id+".lock")); if err != nil { return Record{}, err }
	if !live || !processAlive(record.PID) { cleanupStale(dir, id); return Record{}, fmt.Errorf("session %s is not running", id) }
	return record, nil
}

func leaseHeld(path string) (bool, error) {
	lock, ok, err := filelock.TryAcquire(path, filelock.Exclusive)
	if err != nil { return false, err }
	if !ok { return true, nil }
	if err := lock.Close(); err != nil { return false, err }
	return false, nil
}

func cleanupStale(dir, id string) {
	_ = os.Remove(filepath.Join(dir, id+".json"))
	_ = os.Remove(filepath.Join(dir, id+".lock"))
}

func SanitizeCallbacks(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "stdout" { out = append(out, value); continue }
		u, err := url.Parse(value)
		if err != nil { out = append(out, "callback"); continue }
		switch u.Scheme {
		case "axiom":
			dataset := u.Host; if dataset == "" { dataset = strings.Trim(u.Path, "/") }
			profile := strings.TrimSpace(u.Query().Get("profile"))
			if profile != "" { out = append(out, "axiom:"+dataset+"@"+profile) } else if domain := strings.TrimSpace(u.Query().Get("domain")); domain != "" { out = append(out, "axiom:"+dataset+"@"+domain) } else { out = append(out, "axiom:"+dataset) }
		case "http", "https":
			u.User = nil; u.RawQuery = ""; u.Fragment = ""; out = append(out, u.String())
		default:
			out = append(out, u.Scheme)
		}
	}
	return out
}

func directory() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("SMOKE_SESSION_DIR")); dir != "" { return filepath.Abs(dir) }
	base, err := os.UserCacheDir(); if err != nil { return "", fmt.Errorf("user cache directory: %w", err) }
	return filepath.Join(base, "smoke", "logmash", "sessions"), nil
}

func newID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil { return "", fmt.Errorf("session id: %w", err) }
	return hex.EncodeToString(b[:]), nil
}

func writeAtomic(path string, record Record) error {
	body, err := json.MarshalIndent(record, "", "  "); if err != nil { return err }
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-*.tmp"); if err != nil { return err }
	tmpPath := tmp.Name(); defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil { _ = tmp.Close(); return err }
	if _, err := tmp.Write(append(body, '\n')); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	return os.Rename(tmpPath, path)
}

func read(path string) (Record, error) {
	body, err := os.ReadFile(path); if err != nil { return Record{}, err }
	var record Record
	if err := json.Unmarshal(body, &record); err != nil { return Record{}, err }
	if record.ID == "" || record.PID <= 0 { return Record{}, fmt.Errorf("invalid session record") }
	return record, nil
}
