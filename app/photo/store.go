package photo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/torbenschinke/eventprint/pkg/diskfree"
)

// Originals ist die Ablage der unveränderten Bilddateien.
//
// Sie ist bewusst ein schlichtes Verzeichnis und kein Blob-Store: Die Dateien
// sind das, was nach einer Feier auf den USB-Stick geht, und wer die
// Speicherkarte in einen Rechner steckt, soll sie dort finden.
type Originals interface {
	// Put legt die Datei name mit dem Inhalt data ab.
	Put(name string, data []byte) error

	// Open liest eine abgelegte Datei.
	Open(name string) (io.ReadCloser, error)

	// Path liefert den absoluten Pfad einer abgelegten Datei.
	Path(name string) string

	// Remove löscht eine Datei. Eine fehlende Datei ist kein Fehler.
	Remove(name string) error

	// Usage beschreibt die Belegung des Datenträgers.
	Usage() (Usage, error)
}

// Usage ist die Speicherbelegung für die Anzeige.
type Usage struct {
	// Total und Free beschreiben den ganzen Datenträger.
	Total, Free int64

	// Photos ist der Platz, den die Originale belegen.
	Photos int64

	// Files ist die Zahl der Originale.
	Files int
}

// DirOriginals legt die Originale in einem Verzeichnis ab.
type DirOriginals struct {
	dir string
}

// NewDirOriginals legt das Verzeichnis bei Bedarf an.
func NewDirOriginals(dir string) (*DirOriginals, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("cannot create originals dir: %w", err)
	}

	return &DirOriginals{dir: dir}, nil
}

// partialPrefix markiert eine Datei, die gerade geschrieben wird.
//
// Das Gerät wird am Stecker ausgeschaltet. Eine halb geschriebene Datei unter
// ihrem endgültigen Namen sähe beim nächsten Start aus wie ein kaputtes Foto;
// unter diesem Präfix ist sie als Rest erkennbar und wird übergangen.
const partialPrefix = ".partial-"

func (d *DirOriginals) Put(name string, data []byte) error {
	if err := validName(name); err != nil {
		return err
	}

	tmp := filepath.Join(d.dir, partialPrefix+name)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("cannot create original: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("cannot write original: %w", err)
	}

	// Erst auf die Karte, dann umbenennen: Sonst kann nach einem harten
	// Ausschalten eine leere Datei unter dem richtigen Namen liegen.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("cannot sync original: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cannot close original: %w", err)
	}

	return os.Rename(tmp, filepath.Join(d.dir, name))
}

func (d *DirOriginals) Open(name string) (io.ReadCloser, error) {
	if err := validName(name); err != nil {
		return nil, err
	}

	return os.Open(filepath.Join(d.dir, name))
}

func (d *DirOriginals) Path(name string) string {
	return filepath.Join(d.dir, filepath.Base(name))
}

func (d *DirOriginals) Remove(name string) error {
	if err := validName(name); err != nil {
		return err
	}

	err := os.Remove(filepath.Join(d.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func (d *DirOriginals) Usage() (Usage, error) {
	var usage Usage

	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return usage, fmt.Errorf("cannot read originals dir: %w", err)
	}

	for _, e := range entries {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), partialPrefix) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		usage.Files++
		usage.Photos += info.Size()
	}

	disk, err := diskfree.Of(d.dir)
	if err != nil {
		return usage, err
	}

	usage.Total = disk.TotalBytes
	usage.Free = disk.FreeBytes

	return usage, nil
}

// validName verhindert, dass ein Dateiname aus dem Verzeichnis herausführt.
// Die Namen bildet das Paket selbst; die Prüfung ist der Riegel dafür, dass
// das so bleibt.
func validName(name string) error {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid original name %q", name)
	}

	return nil
}
