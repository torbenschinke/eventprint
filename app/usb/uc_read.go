package usb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// Read lädt ein Bild vom Stick, etwa um es zu drucken.
type Read func(subject permission.Auditable, ctx context.Context, path string) ([]byte, error)

// maxReadSize begrenzt ein einzelnes Bild. Ein 50-Megapixel-JPEG hat rund
// 25 MB; was darüber liegt, ist kein Foto zum Drucken, würde aber den
// Speicher des Pi beim Dekodieren sprengen.
const maxReadSize = 60 << 20

// NewRead bindet das Laden an das eingehängte Dateisystem.
//
// Der Pfad kommt von außen. Ohne Prüfung wäre Read ein Weg, beliebige
// Dateien der Fotobox zu lesen, bis hin zu /etc/shadow oder den
// Gerätedateien. Gelesen wird deshalb nur, was nach Auflösen aller
// symbolischen Verknüpfungen unter dem Einhängepunkt eines gerade
// angebotenen Sticks liegt.
func NewRead(runner Runner) Read {
	h := host{runner: runner}

	return func(subject permission.Auditable, ctx context.Context, path string) ([]byte, error) {
		if err := subject.Audit(PermRead); err != nil {
			return nil, err
		}

		if !filepath.IsAbs(path) || !isImageName(path) {
			return nil, fmt.Errorf("%s ist kein Bild auf einem USB-Stick", filepath.Base(path))
		}

		// Symbolische Verknüpfungen auflösen, bevor geprüft wird: Ein Stick
		// mit ext4 kann eine Verknüpfung "foto.jpg -> /etc/shadow" tragen.
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("das Bild %s ist nicht lesbar: %w", filepath.Base(path), err)
		}

		drives, err := h.drives(ctx)
		if err != nil {
			return nil, err
		}

		if !insideMountPoint(resolved, drives) {
			return nil, errors.New("das Bild liegt nicht auf einem angeschlossenen USB-Stick")
		}

		f, err := os.Open(resolved)
		if err != nil {
			return nil, fmt.Errorf("das Bild %s ist nicht lesbar: %w", filepath.Base(path), err)
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			return nil, err
		}

		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("%s ist keine Bilddatei", filepath.Base(path))
		}

		if st.Size() > maxReadSize {
			return nil, fmt.Errorf("das Bild %s ist mit %s zu groß; höchstens %s werden geladen", filepath.Base(path), FormatSize(st.Size()), FormatSize(maxReadSize))
		}

		// Auch beim Lesen begrenzen: Die Größe kann sich zwischen Stat und
		// Lesen ändern.
		data, err := io.ReadAll(io.LimitReader(f, maxReadSize+1))
		if err != nil {
			return nil, fmt.Errorf("das Bild %s ist nicht lesbar: %w", filepath.Base(path), err)
		}

		if len(data) > maxReadSize {
			return nil, fmt.Errorf("das Bild %s ist zu groß", filepath.Base(path))
		}

		return data, nil
	}
}

// insideMountPoint prüft, ob path unter dem Einhängepunkt eines Sticks
// liegt. Auch die Einhängepunkte werden aufgelöst, sonst scheiterte der
// Vergleich an einer Verknüpfung im Pfad des Einhängepunkts selbst.
func insideMountPoint(path string, drives []Drive) bool {
	for _, d := range drives {
		if d.MountPoint == "" {
			continue
		}

		mp, err := filepath.EvalSymlinks(d.MountPoint)
		if err != nil {
			continue
		}

		rel, err := filepath.Rel(mp, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}

		return true
	}

	return false
}
