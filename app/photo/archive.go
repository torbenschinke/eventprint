package photo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Archive sichert das unveränderte Original eines eingehenden Bildes.
//
// Der Sinn liegt außerhalb der Fotobox: Nach der Feier sollen die Bilder
// digital an die Gäste weitergegeben werden. Dafür taugt der Blob-Store des
// Image-Subsystems nicht – dort liegen die Dateien unter technischen
// Schlüsseln und teils EXIF-normalisiert. Das Archiv ist deshalb ein
// gewöhnlicher Ordner mit gewöhnlichen Dateien, den man auf einen USB-Stick
// kopieren kann, ohne die Anwendung zu befragen.
//
// Ein Fehler beim Sichern darf den Import nicht verhindern: Ein nicht
// archiviertes Bild ist ärgerlich, ein abgebrochener Druck auf einer Feier
// ist schlimmer.
type Archive func(id ID, name string, raw []byte) error

// NewDirArchive legt die Originale als Dateien unterhalb von dir ab.
//
// Der Dateiname beginnt mit der Foto-ID. Sie enthält den Zeitstempel in
// Millisekunden, wodurch eine alphabetische Sortierung im Dateimanager
// zugleich die chronologische ist. Danach folgt der ursprüngliche Name, damit
// eine Aufnahme auch außerhalb der Fotobox wiedererkennbar bleibt.
func NewDirArchive(dir string) (Archive, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create photo archive %q: %w", dir, err)
	}

	return func(id ID, name string, raw []byte) error {
		if len(raw) == 0 {
			return nil
		}

		path := filepath.Join(dir, archiveFilename(id, name))

		// Erst vollständig schreiben, dann umbenennen. Ein Stromausfall
		// mitten im Schreiben hinterlässt sonst eine halbe Datei, die beim
		// späteren Verteilen als heiles Bild durchgeht.
		tmp, err := os.CreateTemp(dir, ".partial-*")
		if err != nil {
			return fmt.Errorf("cannot create archive file: %w", err)
		}

		if _, err := tmp.Write(raw); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())

			return fmt.Errorf("cannot write archive file: %w", err)
		}

		// Ohne Sync steht die Datei zwar im Verzeichnis, ihr Inhalt aber
		// womöglich noch im Cache. Genau die Bilder der letzten Minuten wären
		// bei einem harten Ausfall verloren.
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())

			return fmt.Errorf("cannot flush archive file: %w", err)
		}

		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmp.Name())

			return fmt.Errorf("cannot close archive file: %w", err)
		}

		if err := os.Rename(tmp.Name(), path); err != nil {
			_ = os.Remove(tmp.Name())

			return fmt.Errorf("cannot move archive file: %w", err)
		}

		return nil
	}, nil
}

// maxArchiveNameLen begrenzt den übernommenen Teil des Ursprungsnamens.
// Manche Kameras und Messenger liefern sehr lange Namen; zusammen mit der ID
// stößt das auf einigen Dateisystemen an die Grenze von 255 Bytes.
const maxArchiveNameLen = 80

// archiveFilename bildet einen Dateinamen, der auf jedem Dateisystem und in
// jedem Cloud-Speicher unverändert überlebt.
func archiveFilename(id ID, name string) string {
	base := sanitizeArchiveName(filepath.Base(name))

	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		// Ohne Endung öffnet kein Betriebssystem das Bild mit einem Klick.
		ext = ".jpg"
	} else {
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}

	if len(base) > maxArchiveNameLen {
		base = base[:maxArchiveNameLen]
	}

	if base == "" {
		return string(id) + ext
	}

	return string(id) + "_" + base + ext
}

// sanitizeArchiveName ersetzt alles, was in einem Dateinamen Ärger macht.
//
// Behalten werden Buchstaben, Ziffern, Punkt, Bindestrich und Unterstrich.
// Umlaute und Leerzeichen fallen bewusst weg: Die Ordner werden später per
// Stick, Cloud oder Mail weitergereicht, und dabei überlebt nur ASCII
// zuverlässig.
func sanitizeArchiveName(name string) string {
	var b strings.Builder

	for _, r := range name {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	// Führende Punkte würden die Datei auf Unix verstecken.
	return strings.TrimLeft(b.String(), ".")
}
