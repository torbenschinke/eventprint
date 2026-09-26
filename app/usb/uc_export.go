package usb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// Export kopiert Fotos der Fotobox auf einen USB-Stick.
//
// Das ist der Hauptzweck des Pakets: Nach der Feier müssen die Bilder vom
// Gerät, und einen anderen einfachen Weg gibt es nicht. progress darf nil
// sein und wird nach jeder Datei aufgerufen, übersprungene eingeschlossen.
type Export func(subject permission.Auditable, ctx context.Context, cmd ExportCmd, progress func(done, total int)) (ExportReport, error)

// ExportCmd beschreibt einen Export.
type ExportCmd struct {
	// Drive ist die Gerätedatei des Sticks, z. B. "/dev/sda1".
	Drive string

	// Folder ist der Zielordner auf dem Stick, "/" trennt Unterordner. Er
	// wird FAT-tauglich gemacht, siehe [SanitizeFolder].
	Folder string

	Files []File
}

// File ist ein zu kopierendes Foto.
type File struct {
	// Name ist der gewünschte Dateiname auf dem Stick.
	Name string

	// Path ist der Quellpfad auf der Fotobox.
	Path string
}

// ExportReport ist die Quittung eines Exports.
type ExportReport struct {
	Copied  int
	Skipped int

	// Bytes zählt nur tatsächlich kopierte Daten.
	Bytes int64

	// Folder ist der absolute Pfad des Zielordners.
	Folder string
}

// FreeSpace liefert den freien Platz in Byte für das Dateisystem von path.
type FreeSpace func(path string) (int64, error)

// NotEnoughSpaceError meldet einen zu vollen Stick, bevor das Kopieren
// beginnt.
//
// Vorher und nicht erst beim ersten Schreibfehler: Ein halb gefüllter Stick
// mit einem abgeschnittenen Foto am Ende ist schlimmer als gar keiner, weil
// der Gastgeber zu Hause glaubt, alles zu haben.
type NotEnoughSpaceError struct {
	Needed int64
	Free   int64
}

func (e NotEnoughSpaceError) Error() string {
	return fmt.Sprintf("auf dem USB-Stick ist nicht genug Platz: benötigt werden %s, frei sind %s", FormatSize(e.Needed), FormatSize(e.Free))
}

// perFileSlack ist der Verschnitt je Datei. FAT belegt immer ganze Cluster,
// bei großen exFAT-Sticks bis 128 KiB; dazu kommen Verzeichniseinträge. Ohne
// diesen Zuschlag ginge die Platzprüfung bei vielen kleinen Dateien knapp
// durch und das Kopieren scheiterte trotzdem.
const perFileSlack = 128 << 10

// copyBufferSize ist groß genug, dass billige Sticks ganze Blöcke bekommen,
// und klein genug, um auf dem Pi nicht aufzufallen.
const copyBufferSize = 1 << 20

// NewExport bindet den Export an udisksctl und das Dateisystem.
func NewExport(runner Runner, free FreeSpace) Export {
	h := host{runner: runner}

	return func(subject permission.Auditable, ctx context.Context, cmd ExportCmd, progress func(done, total int)) (ExportReport, error) {
		if err := subject.Audit(PermExport); err != nil {
			return ExportReport{}, err
		}

		if progress == nil {
			progress = func(int, int) {}
		}

		d, err := h.find(ctx, cmd.Drive)
		if err != nil {
			return ExportReport{}, err
		}

		if len(cmd.Files) == 0 {
			return ExportReport{}, nil
		}

		mp, err := h.mount(ctx, d)
		if err != nil {
			return ExportReport{}, err
		}

		dir := filepath.Join(mp, filepath.FromSlash(SanitizeFolder(cmd.Folder)))
		report := ExportReport{Folder: dir}

		plan, err := planExport(dir, cmd.Files)
		if err != nil {
			return report, err
		}

		var needed int64
		for _, p := range plan {
			if !p.skip {
				needed += p.size + perFileSlack
			}
		}

		if needed > 0 {
			avail, err := free(mp)
			if err != nil {
				return report, fmt.Errorf("der freie Platz auf dem USB-Stick lässt sich nicht ermitteln: %w", err)
			}

			if needed > avail {
				return report, NotEnoughSpaceError{Needed: needed, Free: avail}
			}
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			return report, fmt.Errorf("der Ordner %s lässt sich auf dem USB-Stick nicht anlegen: %w", SanitizeFolder(cmd.Folder), err)
		}

		total := len(plan)
		progress(0, total)

		for i, p := range plan {
			if p.skip {
				report.Skipped++
			} else {
				if err := copyFile(ctx, p.src, p.dst); err != nil {
					return report, fmt.Errorf("%s ließ sich nicht auf den USB-Stick kopieren: %w", filepath.Base(p.dst), err)
				}

				report.Copied++
				report.Bytes += p.size
			}

			progress(i+1, total)
		}

		// Der Verzeichniseintrag gehört zu den Daten: Ohne ihn sind die
		// Dateien zwar geschrieben, aber nach einem Stromausfall nicht
		// auffindbar. Nicht jedes Dateisystem kann ein Verzeichnis
		// synchronisieren; das Aushängen beim Auswerfen schreibt es dann
		// ohnehin, deshalb ist ein Scheitern hier kein Grund, einen
		// gelungenen Export als gescheitert zu melden.
		if f, err := os.Open(dir); err == nil {
			_ = f.Sync()
			_ = f.Close()
		}

		return report, nil
	}
}

// planned ist eine Datei mit ihrem endgültigen Ziel.
type planned struct {
	src  string
	dst  string
	size int64
	skip bool
}

// planExport legt vor dem ersten Byte fest, was wohin kopiert wird.
//
// Vorher planen, damit die Platzprüfung nur die Dateien zählt, die wirklich
// fehlen. Nach einem abgebrochenen Export ist der Stick meist schon fast
// voll mit genau diesen Bildern; sie alle erneut einzurechnen, würde den
// zweiten Versuch grundlos ablehnen.
func planExport(dir string, files []File) ([]planned, error) {
	// FAT unterscheidet nicht nach Groß- und Kleinschreibung. Zwei Fotos
	// "a.jpg" und "A.JPG" im selben Export würden sich überschreiben.
	taken := map[string]bool{}
	plan := make([]planned, 0, len(files))

	for _, f := range files {
		st, err := os.Stat(f.Path)
		if err != nil {
			return nil, fmt.Errorf("das Foto %s ist auf der Fotobox nicht mehr lesbar: %w", f.Name, err)
		}

		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("%s ist keine Bilddatei", f.Name)
		}

		name := sanitizeFileName(f.Name)
		if name == "" {
			name = sanitizeFileName(filepath.Base(f.Path))
		}

		p, err := placeFile(dir, name, st.Size(), taken)
		if err != nil {
			return nil, err
		}

		p.src = f.Path
		p.size = st.Size()
		plan = append(plan, p)
	}

	return plan, nil
}

// placeFile sucht den Zielnamen für eine Datei.
//
// Nie überschreiben: Auf dem Stick können Bilder einer früheren Feier mit
// demselben Namen liegen. Eine gleich große Datei gleichen Namens gilt als
// bereits kopiert, damit ein nach einem Abbruch wiederholter Export dort
// weitermacht, wo er stehen blieb, statt alles doppelt anzulegen. Sonst wird
// " (2)", " (3)" … angehängt, wie es Windows und macOS auch tun.
func placeFile(dir, name string, size int64, taken map[string]bool) (planned, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	for n := 1; n < 10000; n++ {
		candidate := name
		if n > 1 {
			candidate = stem + " (" + strconv.Itoa(n) + ")" + ext
		}

		key := strings.ToLower(candidate)
		if taken[key] {
			continue
		}

		dst := filepath.Join(dir, candidate)

		st, err := os.Stat(dst)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			taken[key] = true
			return planned{dst: dst}, nil
		case err != nil:
			return planned{}, fmt.Errorf("auf dem USB-Stick lässt sich %s nicht prüfen: %w", candidate, err)
		case st.Mode().IsRegular() && st.Size() == size:
			taken[key] = true
			return planned{dst: dst, skip: true}, nil
		}
	}

	return planned{}, fmt.Errorf("für %s findet sich auf dem USB-Stick kein freier Dateiname", name)
}

// copyFile kopiert über eine versteckte Zwischendatei.
//
// Die Zwischendatei sorgt dafür, dass unter dem richtigen Namen nur fertige
// Fotos liegen. Wird der Stick mittendrin gezogen, bleibt höchstens eine
// ".…part"-Datei zurück, die der nächste Export ersetzt, und kein halbes Bild,
// das die Überspringen-Regel für vollständig halten könnte.
func copyFile(ctx context.Context, src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	st, err := in.Stat()
	if err != nil {
		return err
	}

	tmp := filepath.Join(filepath.Dir(dst), "."+filepath.Base(dst)+".part")
	_ = os.Remove(tmp)

	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
		}
	}()

	if _, err = io.CopyBuffer(out, ctxReader{ctx: ctx, r: in}, make([]byte, copyBufferSize)); err != nil {
		return err
	}

	// Ohne Sync meldet der Export Erfolg, während die Daten noch im
	// Seitencache des Pi liegen. Wer dann ohne Auswerfen abzieht, hat leere
	// Dateien.
	if err = out.Sync(); err != nil {
		return err
	}

	if err = out.Close(); err != nil {
		return err
	}

	// Das Aufnahmedatum bleibt so im Dateidatum erhalten, und die
	// Bildbetrachter des Gastgebers sortieren richtig.
	_ = os.Chtimes(tmp, st.ModTime(), st.ModTime())

	return os.Rename(tmp, dst)
}

// ctxReader bricht das Kopieren ab, sobald der Kontext endet. Ein großer
// Export auf einen langsamen Stick dauert Minuten; ohne Abbruch liefe er
// weiter, auch wenn die Oberfläche längst aufgegeben hat.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}

	return c.r.Read(p)
}
