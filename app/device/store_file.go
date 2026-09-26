package device

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileSettings hält die Einstellungen als JSON-Datei.
type FileSettings struct {
	path string
}

// NewFileSettings erzeugt den Speicher; die Datei entsteht beim ersten
// Speichern.
func NewFileSettings(path string) *FileSettings {
	return &FileSettings{path: path}
}

func (f *FileSettings) Load() (Settings, error) {
	var s Settings
	ok, err := readJSON(f.path, &s)
	if err != nil {
		return Settings{}, err
	}

	if !ok {
		return DefaultSettings(), nil
	}

	return s, nil
}

func (f *FileSettings) Save(s Settings) error {
	return writeJSON(f.path, s)
}

// FileKiosk hält den Kiosk-Zustand als Datei im Laufzeitverzeichnis.
//
// Unter systemd ist das /run/eventprint: ein tmpfs, das ein Neustart des
// Dienstes überlebt und ein Neustart des Geräts nicht.
type FileKiosk struct {
	path string
}

// NewFileKiosk erzeugt den Speicher.
func NewFileKiosk(path string) *FileKiosk {
	return &FileKiosk{path: path}
}

func (f *FileKiosk) Load() (Kiosk, error) {
	var k Kiosk
	if _, err := readJSON(f.path, &k); err != nil {
		return Kiosk{}, err
	}

	return k, nil
}

func (f *FileKiosk) Save(k Kiosk) error {
	return writeJSON(f.path, k)
}

func (f *FileKiosk) Clear() error {
	err := os.Remove(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func readJSON(path string, dst any) (bool, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("cannot read %s: %w", path, err)
	}

	if err := json.Unmarshal(buf, dst); err != nil {
		return false, fmt.Errorf("cannot parse %s: %w", path, err)
	}

	return true, nil
}

// writeJSON schreibt über eine Zwischendatei, damit ein Ausschalten mitten
// im Schreiben nie eine halbe Einstellungsdatei hinterlässt.
func writeJSON(path string, v any) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	if _, err := f.Write(buf); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}
