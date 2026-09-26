package lightroom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Tokens sind die Schlüssel, mit denen die Box im Namen des Besitzers auf
// Lightroom zugreift.
type Tokens struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// TokenStore bewahrt die Schlüssel über einen Neustart hinweg auf.
//
// Eine Schnittstelle und nicht direkt eine Datei: Die Anwendungsfälle lassen
// sich so ohne Dateisystem prüfen, und falls die Schlüssel später in den
// verschlüsselten Einstellungsspeicher wandern, ändert sich nur der Adapter.
type TokenStore interface {
	// Load liefert die gespeicherten Schlüssel; ok ist false, wenn keine da sind.
	Load() (tokens Tokens, ok bool, err error)
	// Save ersetzt die gespeicherten Schlüssel.
	Save(Tokens) error
	// Delete vergisst die Schlüssel. Fehlen sie bereits, ist das kein Fehler.
	Delete() error
}

// FileTokenStore legt die Schlüssel als JSON-Datei ab.
type FileTokenStore struct {
	path string
}

// NewFileTokenStore speichert die Schlüssel unter path.
func NewFileTokenStore(path string) *FileTokenStore {
	return &FileTokenStore{path: path}
}

func (s *FileTokenStore) Load() (Tokens, bool, error) {
	buf, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return Tokens{}, false, nil
	}

	if err != nil {
		return Tokens{}, false, fmt.Errorf("Lightroom-Schlüssel können nicht gelesen werden: %w", err)
	}

	var t Tokens
	if err := json.Unmarshal(buf, &t); err != nil {
		return Tokens{}, false, fmt.Errorf("Lightroom-Schlüssel sind beschädigt: %w", err)
	}

	if t.AccessToken == "" && t.RefreshToken == "" {
		return Tokens{}, false, nil
	}

	return t, true, nil
}

// Save schreibt zuerst eine temporäre Datei und benennt sie dann um.
//
// Die Box wird einfach ausgesteckt, wenn die Feier vorbei ist. Fällt der Strom
// mitten im Schreiben, bliebe sonst eine halbe Datei zurück – und mit ihr der
// Refresh-Token, der nur durch eine neue Anmeldung auf dem Telefon zu ersetzen
// ist. Die Rechte 0600, weil der Refresh-Token Zugriff auf sämtliche Fotos des
// Besitzers gewährt.
func (s *FileTokenStore) Save(t Tokens) error {
	buf, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("Ablage für Lightroom-Schlüssel kann nicht angelegt werden: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".lightroom-tokens-*")
	if err != nil {
		return fmt.Errorf("Lightroom-Schlüssel können nicht gespeichert werden: %w", err)
	}

	tmpName := tmp.Name()
	committed := false

	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("Lightroom-Schlüssel können nicht geschützt werden: %w", err)
	}

	if _, err := tmp.Write(buf); err != nil {
		return fmt.Errorf("Lightroom-Schlüssel können nicht gespeichert werden: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("Lightroom-Schlüssel können nicht gespeichert werden: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Lightroom-Schlüssel können nicht gespeichert werden: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("Lightroom-Schlüssel können nicht gespeichert werden: %w", err)
	}

	committed = true

	return nil
}

func (s *FileTokenStore) Delete() error {
	err := os.Remove(s.path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("Lightroom-Schlüssel können nicht gelöscht werden: %w", err)
}
