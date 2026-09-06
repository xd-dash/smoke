package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSanitizeCallbacks(t *testing.T) {
	got := SanitizeCallbacks([]string{
		"stdout",
		"https://user:secret@example.com/hook?token=hidden",
		"axiom://events?profile=axiom.logma.sh&token-env=SECRET_NAME",
	})
	want := []string{
		"stdout",
		"https://example.com/hook",
		"axiom:events@axiom.logma.sh",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeCallbacks() = %#v, want %#v", got, want)
	}
}

func TestSessionLeaseDefinesLiveness(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SMOKE_SESSION_DIR", dir)

	handle, record, err := Begin(Record{Callbacks: []string{"stdout"}})
	if err != nil {
		t.Fatal(err)
	}
	if handle == nil {
		t.Fatal("expected session handle")
	}

	records, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("expected live session %s, got %#v", record.ID, records)
	}

	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	records, err = List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("closed lease should not remain live: %#v", records)
	}
}

func TestListRejectsRecordWithoutLeaseEvenForLivePID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SMOKE_SESSION_DIR", dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	record := Record{ID: "stale", PID: os.Getpid(), Callbacks: []string{"stdout"}}
	if err := writeAtomic(filepath.Join(dir, record.ID+".json"), record); err != nil {
		t.Fatal(err)
	}

	records, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("record without owned lease should be stale: %#v", records)
	}
	if _, err := os.Stat(filepath.Join(dir, record.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("stale record was not cleaned up: %v", err)
	}
}
