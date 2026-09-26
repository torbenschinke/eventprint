package photo_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/pkg/heif"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

// Egal woher ein Bild kommt: Abgelegt wird exakt, was geliefert wurde. Kein
// erneutes JPEG-Encoding, kein abgeschnittener EXIF-Block – was auf dem
// USB-Stick landet, soll das sein, was die Kamera geschrieben hat.
func TestImportStoresOriginalByteForByteFromEverySource(t *testing.T) {
	lib := newLibrary(t)

	cases := []struct {
		source photo.Source
		name   string
		data   []byte
		ext    string
		w, h   int
	}{
		{photo.SourceCamera, "IMG_0001.JPG", jpegBytes(t, 60, 40), ".jpg", 60, 40},
		{photo.SourceRelay, "handy.png", pngBytes(t, 30, 50), ".png", 30, 50},
		{photo.SourceLightroom, "export.jpg", jpegBytes(t, 20, 20), ".jpg", 20, 20},
		{photo.SourceUSB, "stick.png", pngBytes(t, 12, 8), ".png", 12, 8},
	}

	for _, c := range cases {
		p := lib.importOne(t, photo.ImportCmd{Name: c.name, Source: c.source, Data: c.data})

		if p.Source != c.source || p.Name != c.name {
			t.Errorf("%s: Source/Name = %q/%q", c.source, p.Source, p.Name)
		}

		if !strings.HasSuffix(p.File, c.ext) || p.Width != c.w || p.Height != c.h {
			t.Errorf("%s: File=%q %dx%d, erwartet *%s %dx%d", c.source, p.File, p.Width, p.Height, c.ext, c.w, c.h)
		}

		if !bytes.Equal(lib.stored(t, p), c.data) {
			t.Errorf("%s: Original wurde verändert", c.source)
		}

		if p.CreatedAt.IsZero() {
			t.Errorf("%s: kein Zeitpunkt", c.source)
		}
	}

	// Die Metadaten überstehen den Weg durch das JSON-Repository.
	all, err := lib.uc.FindAll(su, photo.Query{})
	if err != nil || len(all) != len(cases) {
		t.Fatalf("FindAll = %d, %v", len(all), err)
	}

	spec.Verified(t, foto.RFotoImport)
}

// Ein hochkant gehaltenes Handy liefert quer liegende Pixel und ein EXIF-Tag.
// Die Maße im Foto müssen die aufgerichteten sein – sonst wird hochkant
// gedruckt, was quer gemeint war. Die Datei selbst bleibt dabei unangetastet.
func TestImportHonoursExifOrientationWithoutTouchingTheFile(t *testing.T) {
	lib := newLibrary(t)

	data := jpegWithOrientation(t, 60, 40, 6) // 90° im Uhrzeigersinn
	p := lib.importOne(t, photo.ImportCmd{Source: photo.SourceRelay, Data: data})

	if p.Width != 40 || p.Height != 60 || p.Landscape() {
		t.Fatalf("Maße = %dx%d, erwartet 40x60 hochkant", p.Width, p.Height)
	}

	if !bytes.Equal(lib.stored(t, p), data) {
		t.Fatal("EXIF-Block oder Pixel wurden beim Ablegen verändert")
	}

	spec.Verified(t, foto.RFotoImport)
}

// Handys schicken gern Pfade, Windows-Pfade oder gar keinen Namen.
func TestImportCleansTheDeliveredName(t *testing.T) {
	lib := newLibrary(t)

	cases := map[string]string{
		`C:\Users\gast\DCIM\IMG_7.JPG`: "IMG_7.JPG",
		"/storage/emulated/0/a.jpg":    "a.jpg",
		"ohne-endung":                  "ohne-endung.jpg",
		"   ":                          "",
		"":                             "",
	}

	for in, want := range cases {
		p := lib.importOne(t, photo.ImportCmd{Name: in, Source: photo.SourceUSB})
		if p.Name != want {
			t.Errorf("Name(%q) = %q, erwartet %q", in, p.Name, want)
		}

		// Der Ablagename kommt nie vom Absender.
		if !strings.HasPrefix(p.File, string(p.ID)) {
			t.Errorf("File %q folgt dem Absender statt der ID", p.File)
		}
	}
}

// Was nicht als Bild lesbar ist, darf nicht als Leiche in der Ablage liegen
// bleiben: weder Datei noch Eintrag.
func TestImportRejectsWhatIsNoPhoto(t *testing.T) {
	lib := newLibrary(t)

	bad := map[string][]byte{
		"leer":      nil,
		"text":      []byte("kein Bild, nur Text"),
		"halb-jpeg": jpegBytes(t, 40, 30)[:10],
		"zu groß":   make([]byte, photo.MaxImportBytes+1),
	}

	for name, data := range bad {
		if _, err := lib.uc.Import(su, photo.ImportCmd{Name: name, Source: photo.SourceUSB, Data: data}); err == nil {
			t.Errorf("%s wurde übernommen", name)
		}
	}

	// Ohne Berechtigung auch kein gültiges Bild.
	if _, err := lib.uc.Import(allow(), photo.ImportCmd{Source: photo.SourceUSB, Data: jpegBytes(t, 4, 4)}); err == nil {
		t.Error("Import ohne Berechtigung gelang")
	}

	all, err := lib.uc.FindAll(su, photo.Query{})
	if err != nil || len(all) != 0 {
		t.Fatalf("Mediathek nach Ablehnungen: %d Fotos, %v", len(all), err)
	}

	entries, err := os.ReadDir(lib.dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("Ablage nach Ablehnungen: %d Dateien, %v", len(entries), err)
	}

	spec.Verified(t, foto.RFotoImport)
}

// iPhones schicken HEIC. Abgelegt wird das HEIC selbst, nicht eine
// Umwandlung – die Maße kommen aus libheif.
func TestImportKeepsHEICAsHEIC(t *testing.T) {
	if !heif.Register() {
		t.Skip("libheif ist hier nicht installiert")
	}

	data, err := os.ReadFile("../../pkg/heif/testdata/sample.heic")
	if err != nil {
		t.Fatal(err)
	}

	lib := newLibrary(t)
	p := lib.importOne(t, photo.ImportCmd{Name: "IMG_4711", Source: photo.SourceRelay, Data: data})

	if !strings.HasSuffix(p.File, ".heic") || p.Name != "IMG_4711.heic" {
		t.Fatalf("File/Name = %q/%q", p.File, p.Name)
	}

	if p.Width != 64 || p.Height != 48 {
		t.Fatalf("Maße = %dx%d, erwartet 64x48", p.Width, p.Height)
	}

	if !bytes.Equal(lib.stored(t, p), data) {
		t.Fatal("HEIC wurde verändert")
	}

	spec.Verified(t, foto.RFotoImport)
}
