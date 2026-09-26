package uiphotoupld

import (
	"bytes"
	"fmt"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"slices"
	"strings"
	"testing"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"
	"go.wdy.de/nago/presentation/core"

	"github.com/torbenschinke/eventprint/app/upld"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

// fakeWindow ist ein Fenster, das nur die Adresse und die Dateiauswahl kennt.
//
// Alles andere fehlt absichtlich: Ruft die Seite etwas auf, womit dieser Test
// nicht rechnet, soll er laut scheitern statt still etwas anderes zu prüfen.
type fakeWindow struct {
	core.Window
	values core.Values
	picked []core.File
	opts   []core.ImportFilesOptions
}

func (w *fakeWindow) Values() core.Values { return w.values }

// ImportFiles liefert sofort die Bilder, die der Gast gewählt hätte.
func (w *fakeWindow) ImportFiles(o core.ImportFilesOptions) {
	w.opts = append(w.opts, o)
	o.OnCompletion(w.picked)
}

func newTestOptions(t *testing.T) Options {
	t.Helper()

	images := image.NewUseCases(
		json.NewSloppyJSONRepository[image.SrcSet, image.ID](mem.NewBlobStore("img.set")),
		mem.NewBlobStore("img.blob"),
	)

	return Options{Registry: upld.NewRegistry(nil), CreateSrcSet: images.CreateSrcSet}
}

func testJPEG(t *testing.T) []byte {
	t.Helper()

	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 32, 24))
	for y := range 24 {
		for x := range 32 {
			img.Set(x, y, color.White)
		}
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, nil); err != nil {
		t.Fatalf("JPEG: %v", err)
	}

	return out.Bytes()
}

// TestUploadAddressSelectsInbox hält fest, dass die Adresse im QR-Code die Art
// des Uploads bestimmt: Mit m=inbox erscheint die Eingangsseite, ohne die
// gewohnte Seite mit Layoutwahl.
func TestUploadAddressSelectsInbox(t *testing.T) {
	opts := newTestOptions(t)

	id, err := opts.Registry.Open("box")
	if err != nil {
		t.Fatal(err)
	}

	render := func(values core.Values) string {
		return fmt.Sprintf("%+v", PageUpload(&fakeWindow{values: values}, opts))
	}

	inbox := render(core.Values{"u": string(id), "m": modeInbox})
	if !strings.Contains(inbox, "Fotos an die Box senden") || strings.Contains(inbox, "Dein Foto drucken") {
		t.Fatalf("m=inbox zeigt nicht die Eingangsseite: %s", inbox)
	}

	for _, mode := range []string{"", "print", "INBOX"} {
		page := render(core.Values{"u": string(id), "m": mode})
		if !strings.Contains(page, "Dein Foto drucken") || strings.Contains(page, "Fotos an die Box senden") {
			t.Fatalf("m=%q zeigt nicht die Druckseite: %s", mode, page)
		}
	}

	spec.Verified(t, upload.RUploadEingang)
}

// TestInboxTakesSeveralImagesWithoutLayout hält fest, dass der Eingang
// mehrere Bilder auf einmal annimmt und keines davon ein Layout bekommt – die
// Box druckt es also nicht von selbst.
func TestInboxTakesSeveralImagesWithoutLayout(t *testing.T) {
	opts := newTestOptions(t)

	id, err := opts.Registry.Open("box")
	if err != nil {
		t.Fatal(err)
	}

	data := testJPEG(t)
	wnd := &fakeWindow{values: core.Values{"u": string(id), "m": modeInbox}}

	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		wnd.picked = append(wnd.picked, image.MemFile{Filename: name, MimeTypeHint: "image/jpeg", Bytes: data})
	}

	sent := core.StateOf[int](wnd, "sent")
	importInbox(wnd, opts, id, sent)

	if len(wnd.opts) != 1 || !wnd.opts[0].Multiple {
		t.Fatalf("Dateiauswahl = %+v, erwartet eine Auswahl mehrerer Bilder", wnd.opts)
	}

	if sent.Get() != 3 {
		t.Fatalf("gesendet = %d, erwartet 3", sent.Get())
	}

	jobs, err := opts.Registry.Pending("box")
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, job := range jobs {
		if job.Template != upld.InboxTemplate {
			t.Fatalf("Auftrag %s hat Layout %q, erwartet den Eingang", job.Filename, job.Template)
		}

		names = append(names, job.Filename)
	}

	slices.Sort(names)
	if strings.Join(names, ",") != "a.jpg,b.jpg,c.jpg" {
		t.Fatalf("Aufträge = %v", names)
	}

	spec.Verified(t, upload.RUploadEingang)
}
