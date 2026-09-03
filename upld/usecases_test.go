package upld

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"

	"github.com/torbenschinke/eventprint/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

// box ist eine Fotobox als Aufrufer.
//
// Eingebettet wird der Systemnutzer, weil hier nicht die Rechteprüfung auf dem
// Prüfstand steht, sondern die Trennung der Sitzungen. Überschrieben wird
// deshalb nur die Kennung – und genau an ihr hängt die Sitzung.
type box struct {
	user.Subject
	id user.ID
}

func (b box) ID() user.ID { return b.id }

func newBox(id string) box { return box{Subject: user.SU(), id: user.ID(id)} }

func newTestUseCases(t *testing.T) (UseCases, *Registry, image.UseCases) {
	t.Helper()

	images := image.NewUseCases(
		json.NewSloppyJSONRepository[image.SrcSet, image.ID](mem.NewBlobStore("img.set")),
		mem.NewBlobStore("img.blob"),
	)

	registry := NewRegistry(nil)

	return NewUseCases(registry, images), registry, images
}

// enqueue legt ein Bild ab und hängt einen Auftrag daran.
//
// Das ist der Weg, den ein Gast über die Upload-Seite nimmt; hier abgekürzt
// über die Registry, weil die Oberfläche nicht Gegenstand dieser Zusagen ist.
func enqueue(t *testing.T, registry *Registry, images image.UseCases, id UploadID, name string) JobID {
	t.Helper()

	jobID, err := NewJobID()
	if err != nil {
		t.Fatalf("NewJobID: %v", err)
	}

	set, err := images.CreateSrcSet(user.SU(), image.Options{ID: image.ID(ImagePrefix + string(jobID))}, image.MemFile{
		Filename: name, MimeTypeHint: "image/jpeg", Bytes: testJPEG(t),
	})
	if err != nil {
		t.Fatalf("CreateSrcSet: %v", err)
	}

	job := Job{ID: jobID, Image: set.ID, Template: printing.TemplateFull, Filename: name}
	if err := registry.Enqueue(id, job); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	return jobID
}

// testJPEG liefert ein kleines, gültiges JPEG.
func testJPEG(t *testing.T) []byte {
	t.Helper()

	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			img.Set(x, y, color.Black)
		}
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, nil); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	return out.Bytes()
}

func TestOpenSessionGivesEachBoxExactlyOneAddress(t *testing.T) {
	uc, registry, _ := newTestUseCases(t)

	a := newBox("box-a")

	first, err := uc.OpenSession(a)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}

	if !registry.Valid(first) {
		t.Fatal("die eben geöffnete Sitzung gilt nicht")
	}

	// Dieselbe Fotobox meldet sich erneut an – etwa nach einem Neustart.
	second, err := uc.OpenSession(a)
	if err != nil {
		t.Fatalf("OpenSession (erneut): %v", err)
	}

	if second == first {
		t.Fatal("die zweite Anmeldung lieferte dieselbe Adresse")
	}

	if registry.Valid(first) {
		t.Fatal("die alte Adresse gilt weiterhin – zwei gültige Adressen je Fotobox sind eine zu viel")
	}

	// Eine andere Fotobox bekommt ihre eigene Adresse.
	b := newBox("box-b")

	other, err := uc.OpenSession(b)
	if err != nil {
		t.Fatalf("OpenSession (andere Box): %v", err)
	}

	if other == second {
		t.Fatal("zwei Fotoboxen teilen sich eine Adresse")
	}

	if !registry.Valid(second) {
		t.Fatal("die Anmeldung einer Box hat die Sitzung einer anderen verworfen")
	}

	spec.Verified(t, upload.RUploadSitzung)
}

func TestFindPendingJobsShowsOnlyOwnJobs(t *testing.T) {
	uc, registry, images := newTestUseCases(t)

	a, b := newBox("box-a"), newBox("box-b")

	idA, err := uc.OpenSession(a)
	if err != nil {
		t.Fatalf("OpenSession(a): %v", err)
	}

	if _, err := uc.OpenSession(b); err != nil {
		t.Fatalf("OpenSession(b): %v", err)
	}

	mine := enqueue(t, registry, images, idA, "meins.jpg")

	jobs, err := uc.FindPendingJobs(a)
	if err != nil {
		t.Fatalf("FindPendingJobs(a): %v", err)
	}

	if len(jobs) != 1 || jobs[0].ID != mine {
		t.Fatalf("Box A sieht %d Aufträge, erwartet genau den eigenen", len(jobs))
	}

	// Die fremde Box darf davon nichts sehen.
	foreign, err := uc.FindPendingJobs(b)
	if err != nil {
		t.Fatalf("FindPendingJobs(b): %v", err)
	}

	if len(foreign) != 0 {
		t.Fatalf("Box B sieht %d fremde Aufträge, erwartet 0", len(foreign))
	}

	spec.Verified(t, upload.RUploadAbholung)
}

func TestOpenJobImageDeliversTheOriginal(t *testing.T) {
	uc, registry, images := newTestUseCases(t)

	a, b := newBox("box-a"), newBox("box-b")

	idA, err := uc.OpenSession(a)
	if err != nil {
		t.Fatalf("OpenSession(a): %v", err)
	}

	if _, err := uc.OpenSession(b); err != nil {
		t.Fatalf("OpenSession(b): %v", err)
	}

	job := enqueue(t, registry, images, idA, "bild.jpg")

	reader, err := uc.OpenJobImage(a, job)
	if err != nil {
		t.Fatalf("OpenJobImage: %v", err)
	}

	raw, err := io.ReadAll(reader)
	_ = reader.Close()

	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("das gelieferte Originalbild ist leer")
	}

	// Eine fremde Box darf das Bild nicht laden können.
	if _, err := uc.OpenJobImage(b, job); err == nil {
		t.Fatal("eine fremde Fotobox konnte das Bild laden")
	}

	// Ein unbekannter Auftrag ebenso wenig.
	if _, err := uc.OpenJobImage(a, "gibt-es-nicht"); err == nil {
		t.Fatal("ein unbekannter Auftrag lieferte ein Bild")
	}

	spec.Verified(t, upload.RUploadBild)
}

func TestAckJobKeepsTheJobUntilConfirmed(t *testing.T) {
	uc, registry, images := newTestUseCases(t)

	a := newBox("box-a")

	idA, err := uc.OpenSession(a)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}

	job := enqueue(t, registry, images, idA, "bild.jpg")

	// Das Abrufen allein darf nichts löschen: Scheitert die Übertragung,
	// wäre das Bild sonst verloren.
	if _, err := uc.FindPendingJobs(a); err != nil {
		t.Fatalf("FindPendingJobs: %v", err)
	}

	if _, err := uc.OpenJobImage(a, job); err != nil {
		t.Fatalf("OpenJobImage: %v", err)
	}

	jobs, err := uc.FindPendingJobs(a)
	if err != nil {
		t.Fatalf("FindPendingJobs: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("nach dem Abruf sind %d Aufträge übrig, erwartet 1", len(jobs))
	}

	if err := uc.AckJob(a, job); err != nil {
		t.Fatalf("AckJob: %v", err)
	}

	jobs, err = uc.FindPendingJobs(a)
	if err != nil {
		t.Fatalf("FindPendingJobs: %v", err)
	}

	if len(jobs) != 0 {
		t.Fatalf("nach der Bestätigung sind %d Aufträge übrig, erwartet 0", len(jobs))
	}

	// Zweimal bestätigen ist ein Fehler, kein stiller Erfolg.
	if err := uc.AckJob(a, job); err == nil {
		t.Fatal("eine zweite Bestätigung wurde stillschweigend angenommen")
	}

	spec.Verified(t, upload.RUploadBestaetigung)
}
