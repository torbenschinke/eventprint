package photo_test

import (
	"slices"
	"testing"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

// guest hat, was ein Gast im Kiosk braucht: Fotos der Feier sehen, einzeln
// abrufen und für Vorschau und Druck finden – aber nicht die Mediathek.
func guest() scopedGuest {
	return scopedGuest{subject: allow(photo.PermFindEvent, photo.PermFindByID, photo.PermLocate, photo.PermOpenOriginal), event: "hochzeit"}
}

// scopedGuest ist ein Gast, der an die laufende Feier gebunden ist – so, wie
// das Gerät seine Gäste anlegt.
type scopedGuest struct {
	subject
	event photo.EventID
}

func (g scopedGuest) EventScope() photo.EventID { return g.event }

// Die Schranke zwischen Feier und Mediathek hängt an der Berechtigung, nicht
// an der Oberfläche. Ein Gast, der die Kennung eines privaten Fotos kennt –
// aus einer alten Adresse, einem Screenshot, geraten –, bekommt es trotzdem
// nicht.
func TestGuestSeesOnlyTheRunningEvent(t *testing.T) {
	lib := newLibrary(t)

	private := lib.importOne(t, photo.ImportCmd{Name: "privat.jpg", Source: photo.SourceRelay, Unseen: true})
	own := lib.importOne(t, photo.ImportCmd{Name: "gast.jpg", Source: photo.SourceRelay, Event: "hochzeit"})
	own2 := lib.importOne(t, photo.ImportCmd{Name: "kamera.jpg", Source: photo.SourceCamera, Event: "hochzeit"})
	foreign := lib.importOne(t, photo.ImportCmd{Name: "andere.jpg", Source: photo.SourceCamera, Event: "geburtstag"})

	g := guest()

	list, err := lib.uc.FindEvent(g, "hochzeit", 0)
	if err != nil {
		t.Fatal(err)
	}

	if want := []photo.ID{own2.ID, own.ID}; !slices.Equal(ids(list), want) {
		t.Fatalf("FindEvent = %v, erwartet %v", ids(list), want)
	}

	// Ohne Feier wäre die Abfrage "alle privaten Fotos".
	if _, err := lib.uc.FindEvent(g, "", 0); err == nil {
		t.Fatal("FindEvent ohne Feier gelang")
	}

	// Die Mediathek als Ganzes bleibt zu.
	if _, err := lib.uc.FindAll(g, photo.Query{Scope: photo.ScopeInbox}); err == nil {
		t.Fatal("Gast konnte die Mediathek lesen")
	}

	// Auch mit bekannter Kennung kein privates Foto.
	if _, ok, err := lib.uc.FindByID(g, private.ID); ok || err != nil {
		t.Fatalf("FindByID(privat) für Gast: ok=%v err=%v", ok, err)
	}

	if _, ok, err := lib.uc.FindByID(g, own.ID); !ok || err != nil {
		t.Fatalf("FindByID(Feier) für Gast: ok=%v err=%v", ok, err)
	}

	// Auch die Fotos einer früheren Feier bleiben verborgen.
	if _, ok, err := lib.uc.FindByID(g, foreign.ID); ok || err != nil {
		t.Fatalf("FindByID(andere Feier) für Gast: ok=%v err=%v", ok, err)
	}

	if locs, _ := lib.uc.Locate(g, foreign.ID); len(locs) != 0 {
		t.Fatal("Locate lieferte einem Gast ein Foto einer anderen Feier")
	}

	locs, err := lib.uc.Locate(g, private.ID, own.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(locs) != 1 || locs[0].Photo.ID != own.ID {
		t.Fatalf("Locate für Gast lieferte %d Orte, erwartet nur das Feierfoto", len(locs))
	}

	// Der Besitzer sieht natürlich alles.
	if _, ok, _ := lib.uc.FindByID(su, private.ID); !ok {
		t.Fatal("Besitzer sieht sein privates Foto nicht")
	}

	if locs, _ := lib.uc.Locate(su, private.ID, foreign.ID); len(locs) != 2 {
		t.Fatalf("Locate für Besitzer = %d, erwartet 2", len(locs))
	}

	spec.Verified(t, modus.RModusPrivat)
}

// Ohne diese Auskunft merkt niemand, dass die Karte voll wird – bis mitten
// auf einer Feier kein Foto mehr angenommen wird.
func TestInspectStorageReportsPhotosAndDisk(t *testing.T) {
	lib := newLibrary(t)

	a := jpegBytes(t, 50, 40)
	b := pngBytes(t, 30, 30)
	lib.importOne(t, photo.ImportCmd{Source: photo.SourceUSB, Data: a})
	lib.importOne(t, photo.ImportCmd{Source: photo.SourceUSB, Data: b})

	usage, err := lib.uc.InspectStorage(su)
	if err != nil {
		t.Fatal(err)
	}

	if usage.Files != 2 || usage.Photos != int64(len(a)+len(b)) {
		t.Fatalf("Files=%d Photos=%d, erwartet 2 und %d", usage.Files, usage.Photos, len(a)+len(b))
	}

	if usage.Total <= 0 || usage.Free <= 0 || usage.Free > usage.Total {
		t.Fatalf("Total=%d Free=%d", usage.Total, usage.Free)
	}

	if _, err := lib.uc.InspectStorage(guest()); err == nil {
		t.Fatal("InspectStorage ohne Berechtigung gelang")
	}

	spec.Verified(t, archiv.RArchivPlatz)
}

// Nach der Feier: alles von ihr weg, samt Dateien. Private Fotos und die
// anderer Feiern bleiben – ein Abschluss darf nie die Mediathek treffen.
func TestPurgeEventRemovesOnlyThatEvent(t *testing.T) {
	lib := newLibrary(t)

	private := lib.importOne(t, photo.ImportCmd{Source: photo.SourceCamera})
	gone1 := lib.importOne(t, photo.ImportCmd{Source: photo.SourceRelay, Event: "hochzeit"})
	gone2 := lib.importOne(t, photo.ImportCmd{Source: photo.SourceCamera, Event: "hochzeit"})
	other := lib.importOne(t, photo.ImportCmd{Source: photo.SourceCamera, Event: "geburtstag"})

	if _, err := lib.uc.PurgeEvent(su, ""); err == nil {
		t.Fatal("PurgeEvent ohne Feier gelang")
	}

	if _, err := lib.uc.PurgeEvent(guest(), "hochzeit"); err == nil {
		t.Fatal("PurgeEvent ohne Berechtigung gelang")
	}

	n, err := lib.uc.PurgeEvent(su, "hochzeit")
	if err != nil || n != 2 {
		t.Fatalf("PurgeEvent = %d, %v", n, err)
	}

	for _, p := range []photo.Photo{gone1, gone2} {
		if lib.exists(p) {
			t.Fatalf("Original %s liegt noch in der Ablage", p.File)
		}
		if _, ok, _ := lib.uc.FindByID(su, p.ID); ok {
			t.Fatalf("Eintrag %s existiert noch", p.ID)
		}
	}

	for _, p := range []photo.Photo{private, other} {
		if !lib.exists(p) {
			t.Fatalf("Original %s wurde mitgelöscht", p.File)
		}
	}

	all, err := lib.uc.FindAll(su, photo.Query{})
	if want := []photo.ID{other.ID, private.ID}; err != nil || !slices.Equal(ids(all), want) {
		t.Fatalf("Mediathek danach = %v, erwartet %v (%v)", ids(all), want, err)
	}

	// Ein zweiter Abschluss findet nichts mehr.
	if n, err := lib.uc.PurgeEvent(su, "hochzeit"); n != 0 || err != nil {
		t.Fatalf("zweiter PurgeEvent = %d, %v", n, err)
	}

	spec.Verified(t, archiv.RArchivLoeschen)
}
