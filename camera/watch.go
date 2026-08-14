// Package camera bindet die Kamera an die Fotobox an.
//
// Die Kamera hängt per USB am Rechner und wird über PTP/MTP angesprochen.
// Statt das Protokoll selbst zu implementieren, setzt die Fotobox auf ein
// Übergabeverzeichnis: gphoto2 legt im Tethering-Betrieb jede Aufnahme dort
// ab
//
//	gphoto2 --capture-tethered --filename '<dir>/%Y%m%d-%H%M%S.jpg'
//
// und dieses Paket übernimmt sie von dort. Das entkoppelt die Anwendung
// vollständig vom Kameramodell und erlaubt es, dieselbe Schnittstelle später
// für einen echten PTP-Treiber zu verwenden.
package camera

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/photo"
	"github.com/torbenschinke/eventprint/printing"
)

// Options steuern die Übernahme von Kamerabildern.
type Options struct {
	// Dir ist das Übergabeverzeichnis. Ist es leer, bleibt die
	// Kameraanbindung deaktiviert.
	Dir string

	// Interval bestimmt, wie oft das Verzeichnis geprüft wird.
	// Null bedeutet eine Sekunde.
	Interval time.Duration

	// AutoPrint druckt jede Aufnahme sofort mit AutoPrintTemplate. Das ist
	// der klassische Fotobox-Betrieb: auslösen, warten, Bild mitnehmen.
	AutoPrint bool

	// AutoPrintTemplate ist das Layout für den Sofortdruck.
	AutoPrintTemplate printing.TemplateID

	// Delete entfernt die Datei nach erfolgreicher Übernahme. Die Bilddaten
	// liegen dann ausschließlich im Blob-Store der Anwendung.
	Delete bool
}

// Watch übernimmt fortlaufend neue Kamerabilder, bis ctx abgebrochen wird.
//
// Ein Polling von einer Sekunde ist hier einem Dateisystem-Watcher deutlich
// überlegen: inotify meldet bereits das Anlegen der Datei, während die Kamera
// noch schreibt. Das Polling prüft dagegen, ob die Dateigröße stabil ist, und
// vermeidet so halb übertragene JPEGs.
func Watch(ctx context.Context, opts Options, photos photo.UseCases, printer printing.UseCases) {
	if strings.TrimSpace(opts.Dir) == "" {
		slog.Info("camera ingest disabled, no directory configured")
		return
	}

	interval := opts.Interval
	if interval <= 0 {
		interval = time.Second
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		slog.Error("cannot create camera directory", "dir", opts.Dir, "err", err)
		return
	}

	slog.Info("watching camera directory", "dir", opts.Dir, "autoPrint", opts.AutoPrint)

	w := &watcher{opts: opts, photos: photos, printer: printer, sizes: map[string]int64{}}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("camera watcher stopped")
			return

		case <-ticker.C:
			w.scan()
		}
	}
}

type watcher struct {
	opts    Options
	photos  photo.UseCases
	printer printing.UseCases

	// sizes merkt sich die Größe aus dem vorherigen Durchlauf, um eine noch
	// wachsende Datei zu erkennen.
	sizes map[string]int64

	// seen verhindert eine erneute Übernahme, falls die Datei nicht gelöscht
	// werden soll.
	seen map[string]bool
}

func (w *watcher) scan() {
	entries, err := os.ReadDir(w.opts.Dir)
	if err != nil {
		slog.Error("cannot read camera directory", "dir", w.opts.Dir, "err", err)
		return
	}

	if w.seen == nil {
		w.seen = map[string]bool{}
	}

	for _, entry := range entries {
		if entry.IsDir() || !isImage(entry.Name()) {
			continue
		}

		path := filepath.Join(w.opts.Dir, entry.Name())
		if w.seen[path] {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Erst übernehmen, wenn die Größe zwischen zwei Durchläufen konstant
		// bleibt – dann ist die Übertragung abgeschlossen.
		if prev, ok := w.sizes[path]; !ok || prev != info.Size() {
			w.sizes[path] = info.Size()
			continue
		}

		delete(w.sizes, path)
		w.ingest(path)
	}
}

func (w *watcher) ingest(path string) {
	buf, err := os.ReadFile(path)
	if err != nil {
		slog.Error("cannot read camera file", "path", path, "err", err)
		return
	}

	// Die Übernahme läuft ohne Nutzerkontext, deshalb als Systemnutzer.
	p, err := w.photos.Import(user.SU(), photo.Options{Source: photo.SourceCamera}, image.MemFile{
		Filename:     filepath.Base(path),
		MimeTypeHint: mimeTypeOf(path),
		Bytes:        buf,
	})
	if err != nil {
		slog.Error("cannot import camera file", "path", path, "err", err)
		return
	}

	slog.Info("imported camera photo", "path", path, "photo", string(p.ID))

	if w.opts.Delete {
		if err := os.Remove(path); err != nil {
			slog.Error("cannot remove camera file", "path", path, "err", err)
			w.seen[path] = true
		}
	} else {
		w.seen[path] = true
	}

	if !w.opts.AutoPrint {
		return
	}

	tpl := w.opts.AutoPrintTemplate
	if tpl == "" {
		tpl = printing.TemplateFull
	}

	if _, err := w.printer.Print(user.SU(), p.ID, tpl); err != nil {
		slog.Error("cannot auto print camera photo", "photo", string(p.ID), "err", err)
	}
}

func isImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func mimeTypeOf(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return "image/png"
	}

	return "image/jpeg"
}
