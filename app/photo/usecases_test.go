package photo

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

// newTestUseCases baut die Anwendungsfälle über echten Speicher im
// Arbeitsspeicher auf.
//
// Bewusst kein Stub für das Image-Subsystem: Der Weg vom eingehenden Bild bis
// zu den Originaldaten für den Druck führt durch dessen Ablage, und genau
// dieser Weg ist es, den die Anforderungen zusagen.
func newTestUseCases(t *testing.T) UseCases {
	t.Helper()

	images := image.NewUseCases(
		json.NewSloppyJSONRepository[image.SrcSet, image.ID](mem.NewBlobStore("img.set")),
		mem.NewBlobStore("img.blob"),
	)

	archive, err := NewDirArchive(t.TempDir())
	if err != nil {
		t.Fatalf("NewDirArchive: %v", err)
	}

	repo := json.NewSloppyJSONRepository[Photo, ID](mem.NewBlobStore("photo"))

	return NewUseCases(events.NewEventBus(), repo, images, archive)
}

// importOne legt ein Bild an und liefert es zurück.
func importOne(t *testing.T, uc UseCases, name string) Photo {
	t.Helper()

	p, err := uc.Import(user.SU(), Options{Source: SourceCamera}, image.MemFile{
		Filename: name, MimeTypeHint: "image/jpeg", Bytes: jpegRotated(t),
	})
	if err != nil {
		t.Fatalf("Import(%q): %v", name, err)
	}

	return p
}

func TestFindByIDReturnsTheImportedPhoto(t *testing.T) {
	uc := newTestUseCases(t)

	want := importOne(t, uc, "a.jpg")

	opt, err := uc.FindByID(user.SU(), want.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if opt.IsNone() {
		t.Fatal("das gerade angelegte Bild wurde nicht gefunden")
	}

	if got := opt.Unwrap(); got.ID != want.ID || got.Name != "a.jpg" {
		t.Fatalf("gefunden %+v, erwartet %+v", got, want)
	}

	// Eine unbekannte Kennung ist kein Fehler, sondern eine leere Antwort.
	missing, err := uc.FindByID(user.SU(), "gibt-es-nicht")
	if err != nil {
		t.Fatalf("FindByID(unbekannt): %v", err)
	}

	if missing.IsSome() {
		t.Fatal("eine unbekannte Kennung lieferte ein Bild")
	}

	spec.Verified(t, foto.RFotoEinzelbild)
}

func TestHistoryListsNewestFirst(t *testing.T) {
	uc := newTestUseCases(t)

	// Die Kennung löst auf Millisekunden auf. Drei Bilder im selben
	// Augenblick hätten keine definierte Reihenfolge zueinander – auf einer
	// Feier liegen zwischen zwei Aufnahmen Sekunden, nicht Bruchteile davon.
	first := importOne(t, uc, "erst.jpg")
	time.Sleep(2 * time.Millisecond)
	second := importOne(t, uc, "dann.jpg")
	time.Sleep(2 * time.Millisecond)
	third := importOne(t, uc, "zuletzt.jpg")

	seq, err := uc.FindAll(user.SU())
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	var all []Photo
	for p, err := range seq {
		if err != nil {
			t.Fatalf("FindAll: %v", err)
		}

		all = append(all, p)
	}

	if len(all) != 3 {
		t.Fatalf("FindAll lieferte %d Bilder, erwartet 3", len(all))
	}

	// Die Kennung trägt den Zeitstempel, die Reihenfolge ist deshalb keine
	// Zufälligkeit der Ablage, sondern eine Zusage.
	if all[0].ID != third.ID || all[2].ID != first.ID {
		t.Fatalf("Reihenfolge %v/%v/%v, erwartet neueste zuerst", all[0].Name, all[1].Name, all[2].Name)
	}

	latest, err := uc.FindLatest(user.SU(), 2)
	if err != nil {
		t.Fatalf("FindLatest: %v", err)
	}

	if len(latest) != 2 {
		t.Fatalf("FindLatest(2) lieferte %d Bilder, erwartet 2", len(latest))
	}

	if latest[0].ID != third.ID || latest[1].ID != second.ID {
		t.Fatal("FindLatest lieferte nicht die beiden jüngsten Bilder")
	}

	spec.Verified(t, foto.RFotoHistorie)
}

func TestDeleteRemovesPhotoFromHistory(t *testing.T) {
	uc := newTestUseCases(t)

	keep := importOne(t, uc, "bleibt.jpg")
	drop := importOne(t, uc, "weg.jpg")

	if err := uc.Delete(user.SU(), drop.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	opt, err := uc.FindByID(user.SU(), drop.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if opt.IsSome() {
		t.Fatal("das gelöschte Bild ist weiterhin auffindbar")
	}

	remaining, err := uc.FindLatest(user.SU(), 10)
	if err != nil {
		t.Fatalf("FindLatest: %v", err)
	}

	if len(remaining) != 1 || remaining[0].ID != keep.ID {
		t.Fatal("das Löschen hat den falschen Bestand hinterlassen")
	}

	spec.Verified(t, foto.RFotoLoeschen)
}

func TestOpenOriginalDeliversThePrintSource(t *testing.T) {
	uc := newTestUseCases(t)

	p := importOne(t, uc, "druck.jpg")

	opt, err := uc.OpenOriginal(user.SU(), p.ID)
	if err != nil {
		t.Fatalf("OpenOriginal: %v", err)
	}

	if opt.IsNone() {
		t.Fatal("zu einem vorhandenen Bild fehlen die Originaldaten")
	}

	reader := opt.Unwrap()
	defer func() { _ = reader.Close() }()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	// Es muss ein lesbares JPEG herauskommen, keine leere oder halbe Datei.
	if len(got) == 0 || !bytes.HasPrefix(got, []byte{0xFF, 0xD8}) {
		t.Fatalf("die Vorlage für den Druck ist kein JPEG (%d Bytes)", len(got))
	}

	// Zu einem unbekannten Bild gibt es keine Vorlage.
	missing, err := uc.OpenOriginal(user.SU(), "gibt-es-nicht")
	if err == nil && missing.IsSome() {
		t.Fatal("zu einer unbekannten Kennung wurden Daten geliefert")
	}

	spec.Verified(t, foto.RFotoDruckvorlage)
}
