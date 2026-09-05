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
)

// Record is lightweight local supervision metadata for one running unattended
// Logmash process. It is not durable Logma graph state.
type Record struct {
	ID           string    `json:"id"`
	PID          int       `json:"pid"`
	Profile      string    `json:"profile"`
	Target       string    `json:"target"`
	Channels     []string  `json:"channels,omitempty"`
	Patterns     []string  `json:"patterns,omitempty"`
	Callbacks    []string  `json:"callbacks"`
	AuthProvider string    `json:"auth_provider,omitempty"`
	StartedAt    time.Time `json:"started_at"`
}

type Handle struct {
	path string
}

func Begin(record Record) (*Handle, Record, error) {
	dir, err := directory()
	if err != nil {
		return nil, Record{}, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, Record{}, fmt.Errorf("create session directory: %w", err)
	}
	id, err := newID()
	if err != nil {
		return nil, Record{}, err
	}
	record.ID = id
	record.PID = os.Getpid()
	record.StartedAt = time.Now().UTC()
	record.Callbacks = SanitizeCallbacks(record.Callbacks)
	path := filepath.Join(dir, id+".json")
	if err := writeAtomic(path, record); err != nil {
		return nil, Record{}, err
	}
	return &Handle{path: path}, record, nil
}

func (h *Handle) Close() error {
	if h == nil || h.path == "" {
		return nil
	}
	err := os.Remove(h.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func List() ([]Record, error) {
	dir, err := directory()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read session directory: %w", err)
	}
	var records []Record
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		record, err := read(path)
		if err != nil {
			continue
		}
		if !processAlive(record.PID) {
			_ = os.Remove(path)
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt.Before(records[j].StartedAt) })
	return records, nil
}

func Stop(id string) (Record, error) {
	record, err := runningRecord(id)
	if err != nil {
		return Record{}, err
	}
	if err := stopProcess(record.PID); err != nil {
		return Record{}, fmt.Errorf("stop session %s: %w", id, err)
	}
	return record, nil
}

func runningRecord(id string) (Record, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, `/\\`) {
		return Record{}, fmt.Errorf("invalid session id")
	}
	dir, err := directory()
	if err != nil {
		return Record{}, err
	}
	path := filepath.Join(dir, id+".json")
	record, err := read(path)
	if err != nil {
		return Record{}, err
	}
	if !processAlive(record.PID) {
		_ = os.Remove(path)
		return Record{}, fmt.Errorf("session %s is not running", id)
	}
	return record, nil
}

func SanitizeCallbacks(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "stdout" {
			out = append(out, value)
			continue
		}
		u, err := url.Parse(value)
		if err != nil {
			out = append(out, "callback")
			continue
		}
		switch u.Scheme {
		case "axiom":
			dataset := u.Host
			if dataset == "" {
				dataset = strings.Trim(u.Path, "/")
			}
			profile := strings.TrimSpace(u.Query().Get("profile"))
			if profile != "" {
				out = append(out, "axiom:"+dataset+"@"+profile)
			} else if domain := strings.TrimSpace(u.Query().Get("domain")); domain != "" {
				out = append(out, "axiom:"+dataset+"@"+domain)
			} else {
				out = append(out, "axiom:"+dataset)
			}
		case "http", "https":
			u.User = nil
			u.RawQuery = ""
			u.Fragment = ""
			out = append(out, u.String())
		default:
			out = append(out, u.Scheme)
		}
	}
	return out
}

func directory() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache directory: %w", err)
	}
	return filepath.Join(base, "smoke", "logmash", "sessions"), nil
}

func newID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("session id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func writeAtomic(path string, record Record) error {
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func read(path string) (Record, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(body, &record); err != nil {
		return Record{}, err
	}
	if record.ID == "" || record.PID <= 0 {
		return Record{}, fmt.Errorf("invalid session record")
	}
	return record, nil
}
