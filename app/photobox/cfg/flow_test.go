package cfgphotobox

import (
	"bytes"
	"context"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

// Der Gast-Ablauf, so weit er ohne Browser prüfbar ist: Ein Bild kommt an, es
// wird gedruckt, der Auftrag wird fertig, und das Bild steht in der Historie.
//
// Der Browsertest deckt weiterhin ab, was nur er kann – Dateiauswahl, Dialog,
// das Zeichnen der Seiten. Alles darunter ist eine Frage an den Go-Code und
// beantwortet sich hier in Millisekunden statt in anderthalb Minuten.

// discardPrinter nimmt jeden Auftrag an und meldet ihn als gedruckt. Das
// entspricht dem Testbetrieb, in dem auch die Browsertests laufen.
type discardPrinter struct{ printed int }

func (p *discardPrinter) Name() string { return "Testdrucker" }

func (p *discardPrinter) Print(context.Context, []byte, string) (printing.Result, error) {
	p.printed++
	return printing.Result{JobID: "Test-1", Message: "angenommen"}, nil
}

func (p *discardPrinter) Await(context.Context, string) printing.Outcome {
	return printing.Outcome{Done: true, Success: true, Reason: "job-completed-successfully"}
}

func (p *discardPrinter) Status(context.Context) printing.PrinterStatus {
	return printing.PrinterStatus{Queue: "Testdrucker", Exists: true, Enabled: true, Accepting: true}
}

// boothStack verdrahtet beide Kontexte so, wie es Enable im Betrieb tut.
func boothStack(t *testing.T) (photo.UseCases, printing.UseCases, *discardPrinter) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	images := image.NewUseCases(
		json.NewSloppyJSONRepository[image.SrcSet, image.ID](mem.NewBlobStore("img.set")),
		mem.NewBlobStore("img.blob"),
	)

	archive, err := photo.NewDirArchive(t.TempDir())
	if err != nil {
		t.Fatalf("NewDirArchive: %v", err)
	}

	bus := events.NewEventBus()

	photos := photo.NewUseCases(bus,
		json.NewSloppyJSONRepository[photo.Photo, photo.ID](mem.NewBlobStore("photo")),
		images, archive, "", nil)

	printer := &discardPrinter{}

	prints := printing.NewUseCases(ctx, bus,
		json.NewSloppyJSONRepository[printing.Job, printing.JobID](mem.NewBlobStore("job")),
		printer, photos.OpenOriginal, nil)

	return photos, prints, printer
}

func sampleJPEG(t *testing.T) []byte {
	t.Helper()

	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 240, 160))
	for y := range 160 {
		for x := range 240 {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, nil); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	return out.Bytes()
}

// awaitJob wartet, bis der Auftrag abgeschlossen ist.
func awaitJob(t *testing.T, prints printing.UseCases, id printing.JobID) printing.Job {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		opt, err := prints.FindJobByID(user.SU(), id)
		if err != nil {
			t.Fatalf("FindJobByID: %v", err)
		}

		if opt.IsSome() && opt.Unwrap().State.Done() {
			return opt.Unwrap()
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("der Auftrag wurde nicht abgeschlossen")

	return printing.Job{}
}

// TestGuestUploadReachesPaperAndHistory ist der Ablauf eines Abends in einem
// Test: Ein Gast lädt hoch, wählt ein Layout, druckt – und findet sein Bild
// danach in der Historie wieder.
func TestGuestUploadReachesPaperAndHistory(t *testing.T) {
	photos, prints, printer := boothStack(t)

	p, err := photos.Import(user.SU(), photo.Options{Source: photo.SourceUpload}, image.MemFile{
		Filename: "gast.jpg", MimeTypeHint: "image/jpeg", Bytes: sampleJPEG(t),
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	jobID, err := prints.Print(user.SU(), p.ID, printing.TemplatePolaroid)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	job := awaitJob(t, prints, jobID)

	if job.State != printing.StateDone {
		t.Fatalf("Zustand = %s (%s), erwartet fertig", job.State, job.Message)
	}

	if job.Template != printing.TemplatePolaroid {
		t.Fatalf("Layout = %s, erwartet Polaroid", job.Template)
	}

	if printer.printed != 1 {
		t.Fatalf("es wurden %d Blätter erzeugt, erwartet 1", printer.printed)
	}

	// Das Bild gehört in die Historie, und zwar mit seiner Herkunft.
	latest, err := photos.FindLatest(user.SU(), 10)
	if err != nil {
		t.Fatalf("FindLatest: %v", err)
	}

	if len(latest) != 1 || latest[0].ID != p.ID {
		t.Fatalf("die Historie enthält %d Bilder, erwartet genau das hochgeladene", len(latest))
	}

	if latest[0].Source != photo.SourceUpload {
		t.Fatalf("Herkunft = %s, erwartet %s", latest[0].Source, photo.SourceUpload)
	}
}

// TestReprintUsesTheSamePhotoAgain deckt den Nachdruck vom Startbildschirm
// ab: dasselbe Bild, ein anderes Layout, ein zweites Blatt.
func TestReprintUsesTheSamePhotoAgain(t *testing.T) {
	photos, prints, printer := boothStack(t)

	p, err := photos.Import(user.SU(), photo.Options{Source: photo.SourceCamera}, image.MemFile{
		Filename: "kamera.jpg", MimeTypeHint: "image/jpeg", Bytes: sampleJPEG(t),
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	first, err := prints.Print(user.SU(), p.ID, printing.TemplateFull)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	awaitJob(t, prints, first)

	second, err := prints.Print(user.SU(), p.ID, printing.TemplatePassepartout)
	if err != nil {
		t.Fatalf("Print (Nachdruck): %v", err)
	}

	job := awaitJob(t, prints, second)

	if job.Photo != p.ID {
		t.Fatal("der Nachdruck bezieht sich auf ein anderes Bild")
	}

	if job.Template != printing.TemplatePassepartout {
		t.Fatalf("Layout = %s, erwartet Passepartout", job.Template)
	}

	if printer.printed != 2 {
		t.Fatalf("es wurden %d Blätter erzeugt, erwartet 2", printer.printed)
	}

	// Ein Nachdruck legt kein zweites Bild an.
	all, err := photos.FindLatest(user.SU(), 10)
	if err != nil {
		t.Fatalf("FindLatest: %v", err)
	}

	if len(all) != 1 {
		t.Fatalf("die Historie enthält %d Bilder, erwartet 1", len(all))
	}
}
