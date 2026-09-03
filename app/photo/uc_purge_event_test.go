package photo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
)

// guestSubject ist ein Subject ohne jede Berechtigung.
type guestSubject struct{ user.Subject }

func (guestSubject) Audit(permission.ID) error { return errors.New("Zugriff verweigert") }

func (guestSubject) HasPermission(permission.ID) bool { return false }

// purgeFixture baut eine Fotobox nach einer Feier: Fotos in der Ablage,
// Bilddaten im Blob-Store, Originale im Archiv.
type purgeFixture struct {
	repo       Repository
	dir        string
	geloescht  []image.ID
	purgeErr   error
	bytesPerID int64
}

func newPurgeFixture(t *testing.T, anzahl int) *purgeFixture {
	t.Helper()

	f := &purgeFixture{
		repo:       json.NewSloppyJSONRepository[Photo, ID](mem.NewBlobStore("photo")),
		dir:        t.TempDir(),
		bytesPerID: 1000,
	}

	for i := range anzahl {
		id := ID(string(rune('a' + i)))

		if err := f.repo.Save(Photo{ID: id, Image: image.ID("img-" + string(id))}); err != nil {
			t.Fatalf("cannot save photo: %v", err)
		}

		name := filepath.Join(f.dir, string(id)+"_bild.jpg")
		if err := os.WriteFile(name, bytes.Repeat([]byte("x"), 100), 0o644); err != nil {
			t.Fatalf("cannot write archive file: %v", err)
		}
	}

	return f
}

func (f *purgeFixture) purgeImage(ctx context.Context, id image.ID) (int64, error) {
	if f.purgeErr != nil {
		return 0, f.purgeErr
	}

	f.geloescht = append(f.geloescht, id)

	return f.bytesPerID, nil
}

func (f *purgeFixture) count(t *testing.T) int {
	t.Helper()

	n := 0
	for _, err := range f.repo.All() {
		if err != nil {
			t.Fatalf("cannot iterate: %v", err)
		}

		n++
	}

	return n
}

// TestPurgeEventClearsEverythingThatCostsSpace ist die Zusage der Funktion:
// Nach der Feier ist die Fotobox leer und der Platz wieder da.
func TestPurgeEventClearsEverythingThatCostsSpace(t *testing.T) {
	f := newPurgeFixture(t, 3)

	var mutex sync.Mutex
	purge := NewPurgeEvent(&mutex, events.NewEventBus(), f.repo, f.purgeImage, f.dir)

	result, err := purge(user.SU())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}

	if result.Photos != 3 {
		t.Fatalf("Photos = %d, erwartet 3", result.Photos)
	}

	// Der Blob-Store ist der Punkt, um den es geht: Delete entfernt bewusst nur
	// Metadaten und gibt keinen Platz frei. Diese Funktion muss es tun.
	if len(f.geloescht) != 3 {
		t.Fatalf("%d Bilddaten geloescht, erwartet 3", len(f.geloescht))
	}

	if result.ImageBytes != 3000 {
		t.Fatalf("ImageBytes = %d, erwartet 3000", result.ImageBytes)
	}

	if result.Archive.Files != 3 || result.Archive.Bytes != 300 {
		t.Fatalf("Archiv = %+v, erwartet 3 Dateien / 300 Bytes", result.Archive)
	}

	if result.FreedBytes() != 3300 {
		t.Fatalf("FreedBytes = %d, erwartet 3300", result.FreedBytes())
	}

	// Die Oberflaeche liest aus der Ablage. Bliebe dort etwas stehen, zeigte
	// die Galerie Bilder, deren Daten fort sind.
	if n := f.count(t); n != 0 {
		t.Fatalf("%d Fotos in der Ablage uebrig, erwartet 0", n)
	}

	rest, err := os.ReadDir(f.dir)
	if err != nil {
		t.Fatalf("cannot read dir: %v", err)
	}

	if len(rest) != 0 {
		t.Fatalf("%d Archivdateien uebrig, erwartet 0", len(rest))
	}

	spec.Verified(t, archiv.RArchivLoeschen)
}

// TestPurgeEventKeepsTheEntryWhenTheImageSurvives ist die Reihenfolge-Zusage.
//
// Zuerst die Bilddaten, dann der Eintrag: Andersherum entstuende ein Blob, den
// niemand mehr findet und der bis zum Neuaufsetzen Platz belegt.
func TestPurgeEventKeepsTheEntryWhenTheImageSurvives(t *testing.T) {
	f := newPurgeFixture(t, 2)
	f.purgeErr = errors.New("Ablage nicht erreichbar")

	var mutex sync.Mutex
	purge := NewPurgeEvent(&mutex, events.NewEventBus(), f.repo, f.purgeImage, f.dir)

	if _, err := purge(user.SU()); err == nil {
		t.Fatal("der Fehler beim Loeschen der Bilddaten wurde verschluckt")
	}

	if n := f.count(t); n != 2 {
		t.Fatalf("%d Fotos uebrig, erwartet 2 - der Versuch muss wiederholbar bleiben", n)
	}
}

// TestPurgeEventWithoutArchiveIsFine deckt den Betrieb ohne Archivordner ab.
func TestPurgeEventWithoutArchiveIsFine(t *testing.T) {
	f := newPurgeFixture(t, 1)

	var mutex sync.Mutex
	purge := NewPurgeEvent(&mutex, events.NewEventBus(), f.repo, f.purgeImage, "")

	result, err := purge(user.SU())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}

	if result.Photos != 1 || result.Archive.Files != 0 {
		t.Fatalf("result = %+v", result)
	}
}

// TestGuestMayNotWipeTheBox: Ein Gast, der die Feier beenden koennte, waere
// der teuerste denkbare Fehler.
func TestGuestMayNotWipeTheBox(t *testing.T) {
	f := newPurgeFixture(t, 2)

	var mutex sync.Mutex
	purge := NewPurgeEvent(&mutex, events.NewEventBus(), f.repo, f.purgeImage, f.dir)

	if _, err := purge(guestSubject{}); err == nil {
		t.Fatal("ein Gast durfte die Fotobox leerraeumen")
	}

	if n := f.count(t); n != 2 {
		t.Fatalf("%d Fotos uebrig, erwartet 2", n)
	}
}
