package photo_test

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

// Im Heimbetrieb landet, was von Handy und Kamera kommt, im Eingang. Was von
// Lightroom oder vom USB-Stick geholt wurde, hat jemand bewusst ausgewählt
// und gehört nicht dorthin; Fotos einer Feier schon gar nicht.
func TestInboxCollectsPrivatePhotosFromPhoneAndCamera(t *testing.T) {
	lib := newLibrary(t)

	phone := lib.importOne(t, photo.ImportCmd{Name: "handy.jpg", Source: photo.SourceRelay, Unseen: true})
	camera := lib.importOne(t, photo.ImportCmd{Name: "kamera.jpg", Source: photo.SourceCamera, Unseen: true})
	lib.importOne(t, photo.ImportCmd{Name: "lr.jpg", Source: photo.SourceLightroom})
	lib.importOne(t, photo.ImportCmd{Name: "usb.jpg", Source: photo.SourceUSB})
	lib.importOne(t, photo.ImportCmd{Name: "feier.jpg", Source: photo.SourceRelay, Event: "hochzeit", Unseen: true})

	inbox, err := lib.uc.FindAll(su, photo.Query{Scope: photo.ScopeInbox})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := ids(inbox), []photo.ID{camera.ID, phone.ID}; !slices.Equal(got, want) {
		t.Fatalf("Eingang = %v, erwartet %v", got, want)
	}

	// Angekommen heißt nicht gedruckt.
	for _, p := range inbox {
		if !p.Unseen || p.Prints != 0 {
			t.Fatalf("%s: Unseen=%v Prints=%d", p.Name, p.Unseen, p.Prints)
		}
	}

	spec.Verified(t, foto.RFotoEingang)
}

// Neu bleibt ein Foto, bis es für den Druck ausgewählt wird – nicht bis es
// jemand anschaut, und auch nicht nach Tagen. Wer es auswählt, nimmt es aus
// dem Eingang; ein Druck tut das ebenso.
func TestInboxPhotoStaysNewUntilSelected(t *testing.T) {
	lib := newLibrary(t)

	selected := lib.importOne(t, photo.ImportCmd{Source: photo.SourceRelay, Unseen: true})
	printed := lib.importOne(t, photo.ImportCmd{Source: photo.SourceCamera, Unseen: true})
	waiting := lib.importOne(t, photo.ImportCmd{Source: photo.SourceRelay, Unseen: true})

	unseen := func(id photo.ID) bool {
		t.Helper()
		p, ok, err := lib.uc.FindByID(su, id)
		if err != nil || !ok {
			t.Fatalf("FindByID(%s) = %v, %v", id, ok, err)
		}
		return p.Unseen
	}

	if err := lib.uc.MarkSeen(su, selected.ID); err != nil {
		t.Fatal(err)
	}

	if err := lib.uc.MarkPrinted(su, printed.ID); err != nil {
		t.Fatal(err)
	}

	if unseen(selected.ID) || unseen(printed.ID) || !unseen(waiting.ID) {
		t.Fatalf("Unseen: ausgewählt=%v gedruckt=%v wartend=%v",
			unseen(selected.ID), unseen(printed.ID), unseen(waiting.ID))
	}

	// Ohne Berechtigung bleibt das Foto neu.
	if err := lib.uc.MarkSeen(allow(), waiting.ID); err == nil {
		t.Fatal("MarkSeen ohne Berechtigung gelang")
	}

	if !unseen(waiting.ID) {
		t.Fatal("Foto verließ den Eingang ohne Berechtigung")
	}

	// Ein inzwischen gelöschtes Foto ist beim Vermerken kein Fehler.
	if err := lib.uc.MarkSeen(su, "0000000000000-weg"); err != nil {
		t.Fatalf("MarkSeen auf verschwundenes Foto: %v", err)
	}

	spec.Verified(t, foto.RFotoEingang)
}

// Die Mediathek zeigt die neuesten Fotos zuerst, Favoriten und Gedrucktes
// lassen sich getrennt ansehen.
func TestHistoryNewestFirstWithFavoritesAndPrinted(t *testing.T) {
	lib := newLibrary(t)

	first := lib.importOne(t, photo.ImportCmd{Name: "1.jpg", Source: photo.SourceCamera})
	second := lib.importOne(t, photo.ImportCmd{Name: "2.jpg", Source: photo.SourceUSB, Event: "sommerfest"})
	third := lib.importOne(t, photo.ImportCmd{Name: "3.jpg", Source: photo.SourceRelay})

	find := func(scope photo.Scope, limit int) []photo.ID {
		t.Helper()
		list, err := lib.uc.FindAll(su, photo.Query{Scope: scope, Limit: limit})
		if err != nil {
			t.Fatal(err)
		}
		return ids(list)
	}

	if got, want := find(photo.ScopeAll, 0), []photo.ID{third.ID, second.ID, first.ID}; !slices.Equal(got, want) {
		t.Fatalf("Mediathek = %v, erwartet %v", got, want)
	}

	if got, want := find(photo.ScopeAll, 2), []photo.ID{third.ID, second.ID}; !slices.Equal(got, want) {
		t.Fatalf("Limit 2 = %v, erwartet %v", got, want)
	}

	if err := lib.uc.SetFavorite(su, true, first.ID, third.ID); err != nil {
		t.Fatal(err)
	}

	if got, want := find(photo.ScopeFavorites, 0), []photo.ID{third.ID, first.ID}; !slices.Equal(got, want) {
		t.Fatalf("Favoriten = %v, erwartet %v", got, want)
	}

	if err := lib.uc.SetFavorite(su, false, third.ID); err != nil {
		t.Fatal(err)
	}

	if got, want := find(photo.ScopeFavorites, 0), []photo.ID{first.ID}; !slices.Equal(got, want) {
		t.Fatalf("Favoriten nach Zurücknehmen = %v, erwartet %v", got, want)
	}

	if len(find(photo.ScopePrinted, 0)) != 0 {
		t.Fatal("Gedruckt ist vor dem ersten Druck nicht leer")
	}

	// Zwei Blätter mit demselben Foto zählen zweimal.
	if err := lib.uc.MarkPrinted(su, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := lib.uc.MarkPrinted(su, second.ID); err != nil {
		t.Fatal(err)
	}

	if got, want := find(photo.ScopePrinted, 0), []photo.ID{second.ID}; !slices.Equal(got, want) {
		t.Fatalf("Gedruckt = %v, erwartet %v", got, want)
	}

	p, _, err := lib.uc.FindByID(su, second.ID)
	if err != nil || p.Prints != 2 {
		t.Fatalf("Prints = %d, %v", p.Prints, err)
	}

	// Favorit und Druck sind Sache des Besitzers.
	if err := lib.uc.SetFavorite(allow(photo.PermFindAll), true, second.ID); err == nil {
		t.Fatal("SetFavorite ohne Berechtigung gelang")
	}
	if _, err := lib.uc.FindAll(allow(), photo.Query{}); err == nil {
		t.Fatal("FindAll ohne Berechtigung gelang")
	}

	spec.Verified(t, foto.RFotoHistorie)
}

// Gleichzeitige Druckvermerke dürfen sich nicht gegenseitig überschreiben:
// Der Worker vermerkt, während die Oberfläche Favoriten setzt.
func TestConcurrentUpdatesAreNotLost(t *testing.T) {
	lib := newLibrary(t)
	p := lib.importOne(t, photo.ImportCmd{Source: photo.SourceCamera, Unseen: true})

	const n = 20

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := lib.uc.MarkPrinted(su, p.ID); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := lib.uc.SetFavorite(su, i%2 == 0, p.ID); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	got, _, err := lib.uc.FindByID(su, p.ID)
	if err != nil || got.Prints != n || got.Unseen {
		t.Fatalf("Prints=%d Unseen=%v, erwartet %d und false (%v)", got.Prints, got.Unseen, n, err)
	}

	spec.Verified(t, foto.RFotoHistorie)
}

func TestFindByIDReturnsTheSinglePhoto(t *testing.T) {
	lib := newLibrary(t)

	want := lib.importOne(t, photo.ImportCmd{Name: "einzel.png", Source: photo.SourceUSB, Data: pngBytes(t, 10, 20)})
	lib.importOne(t, photo.ImportCmd{Name: "anderes.jpg", Source: photo.SourceUSB})

	got, ok, err := lib.uc.FindByID(su, want.ID)
	if err != nil || !ok {
		t.Fatalf("FindByID = %v, %v", ok, err)
	}

	if got.ID != want.ID || got.Name != "einzel.png" || got.File != want.File || got.Width != 10 || got.Height != 20 {
		t.Fatalf("FindByID = %+v, erwartet %+v", got, want)
	}

	// Eine unbekannte Kennung ist kein Fehler, sondern "gibt es nicht".
	if _, ok, err := lib.uc.FindByID(su, "0000000000000-unbekannt"); ok || err != nil {
		t.Fatalf("unbekannte ID: ok=%v err=%v", ok, err)
	}

	if _, _, err := lib.uc.FindByID(allow(), want.ID); err == nil {
		t.Fatal("FindByID ohne Berechtigung gelang")
	}

	spec.Verified(t, foto.RFotoEinzelbild)
}

// Löschen heißt löschen: Eintrag und Original sind danach weg. Auf einer
// Speicherkarte wäre eine verwaiste Datei Platz, den niemand mehr freigeben
// kann, weil keine Oberfläche sie zeigt.
func TestDeleteRemovesMetadataAndOriginal(t *testing.T) {
	lib := newLibrary(t)

	victim := lib.importOne(t, photo.ImportCmd{Source: photo.SourceCamera})
	other := lib.importOne(t, photo.ImportCmd{Source: photo.SourceCamera})

	// Ohne Berechtigung bleibt alles, wie es ist.
	if err := lib.uc.Delete(allow(photo.PermFindAll), victim.ID); err == nil {
		t.Fatal("Delete ohne Berechtigung gelang")
	}
	if !lib.exists(victim) {
		t.Fatal("Original trotz verweigerter Löschung weg")
	}

	// Eine längst gelöschte Kennung in der Auswahl stört nicht.
	if err := lib.uc.Delete(su, victim.ID, "0000000000000-weg"); err != nil {
		t.Fatal(err)
	}

	if lib.exists(victim) {
		t.Fatal("Original liegt nach dem Löschen noch in der Ablage")
	}

	if _, ok, err := lib.uc.FindByID(su, victim.ID); ok || err != nil {
		t.Fatalf("Eintrag nach dem Löschen: ok=%v err=%v", ok, err)
	}

	if _, err := lib.uc.OpenOriginal(su, victim.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenOriginal nach dem Löschen: %v", err)
	}

	if !lib.exists(other) {
		t.Fatal("ein anderes Foto wurde mitgelöscht")
	}

	usage, err := lib.uc.InspectStorage(su)
	if err != nil || usage.Files != 1 {
		t.Fatalf("Files = %d, %v", usage.Files, err)
	}

	spec.Verified(t, foto.RFotoLoeschen)
}

// Gedruckt wird aus dem Original, nicht aus einer Vorschau. Beide Wege zur
// Datei – der Datenstrom und der Pfad für Vorschau und Export – müssen genau
// die abgelegten Bytes liefern.
func TestOriginalIsThePrintSource(t *testing.T) {
	lib := newLibrary(t)

	data := jpegWithOrientation(t, 80, 60, 6)
	p := lib.importOne(t, photo.ImportCmd{Name: "vorlage.jpg", Source: photo.SourceRelay, Data: data})

	r, err := lib.uc.OpenOriginal(su, p.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(readAll(t, r), data) {
		t.Fatal("OpenOriginal liefert nicht das Original")
	}

	locs, err := lib.uc.Locate(su, p.ID, "0000000000000-weg")
	if err != nil || len(locs) != 1 {
		t.Fatalf("Locate = %d, %v", len(locs), err)
	}

	fromPath, err := os.ReadFile(locs[0].Path)
	if err != nil || !bytes.Equal(fromPath, data) {
		t.Fatalf("Datei unter Locate-Pfad weicht ab (%v)", err)
	}

	if locs[0].Photo.ID != p.ID || !strings.HasSuffix(locs[0].ExportName, "_vorlage.jpg") {
		t.Fatalf("Location = %+v", locs[0])
	}

	if _, err := lib.uc.OpenOriginal(allow(), p.ID); err == nil {
		t.Fatal("OpenOriginal ohne Berechtigung gelang")
	}

	if _, err := lib.uc.OpenOriginal(su, "0000000000000-weg"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenOriginal auf unbekannte ID: %v", err)
	}

	spec.Verified(t, foto.RFotoDruckvorlage)
}
