package lightroom_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/torbenschinke/eventprint/app/lightroom"
)

func TestFileTokenStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "lightroom.json")
	store := lightroom.NewFileTokenStore(path)

	if _, ok, err := store.Load(); ok || err != nil {
		t.Fatalf("leere Ablage: ok=%v err=%v", ok, err)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("Löschen einer fehlenden Datei: %v", err)
	}

	want := lightroom.Tokens{AccessToken: "a", RefreshToken: "r", ExpiresAt: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("Rechte %v, erwartet 0600", perm)
	}

	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporäre Dateien liegen geblieben: %v", entries)
	}

	got, ok, err := store.Load()
	if err != nil || !ok || got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("Load = %+v ok=%v err=%v", got, ok, err)
	}

	want.AccessToken = "b"
	if err := store.Save(want); err != nil {
		t.Fatalf("Überschreiben: %v", err)
	}

	if got, _, _ := store.Load(); got.AccessToken != "b" {
		t.Fatalf("nach dem Überschreiben: %+v", got)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, ok, err := store.Load(); ok || err != nil {
		t.Fatalf("nach dem Löschen: ok=%v err=%v", ok, err)
	}
}

func TestFileTokenStoreRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lightroom.json")
	if err := os.WriteFile(path, []byte("{kaputt"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := lightroom.NewFileTokenStore(path).Load(); err == nil {
		t.Fatal("beschädigte Datei wurde nicht gemeldet")
	}
}
