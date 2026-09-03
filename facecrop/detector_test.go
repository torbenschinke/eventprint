//go:build !nofacecrop

package facecrop

import (
	"image"
	"os"
	"testing"

	"github.com/torbenschinke/eventprint/orient"
)

// DSC02301.jpg ist eine Straßenszene ohne sichtbares Gesicht – die einzige
// Person trägt eine Kapuze und ist abgewandt. Der Test hält damit die
// Fehlalarmschwelle fest: YuNet bewertet dort Rumpf und Muster mit bis zu
// etwa 0.3, und würde [minConfidence] darunter sinken, richtete die Fotobox
// den Polaroid-Ausschnitt auf einen Fehltreffer statt auf ein Gesicht aus.
func TestDetectRejectsFalsePositives(t *testing.T) {
	raw, err := os.ReadFile("../DSC02301.jpg")
	if err != nil {
		t.Skipf("Beispielbild nicht vorhanden: %v", err)
	}

	img, _, err := orient.Decode(raw)
	if err != nil {
		t.Fatalf("Beispielbild dekodieren: %v", err)
	}

	faces, err := shared.detect(img)
	if err != nil {
		t.Fatalf("YuNet-Erkennung: %v", err)
	}

	if len(faces) != 0 {
		t.Errorf("Erkennung meldet %d Gesichter in einer Szene ohne Gesicht: %v", len(faces), faces)
	}
}

// Sicherheitsnetz für die Rückrechnung auf die volle Auflösung: Ein Treffer
// darf das Bild niemals verlassen, sonst entstünde ein ungültiger Ausschnitt.
func TestDetectReturnsRectanglesInsideBounds(t *testing.T) {
	raw, err := os.ReadFile("../DSC02301.jpg")
	if err != nil {
		t.Skipf("Beispielbild nicht vorhanden: %v", err)
	}

	img, _, err := orient.Decode(raw)
	if err != nil {
		t.Fatalf("Beispielbild dekodieren: %v", err)
	}

	for _, face := range Detect(img) {
		if face.Empty() || !face.In(img.Bounds()) {
			t.Errorf("Gesichtsrechteck %v liegt nicht im Bild %v", face, img.Bounds())
		}
	}
}

// Ein leeres Bild darf weder den Detektor noch den Druck stören.
func TestDetectHandlesEmptyImage(t *testing.T) {
	if got := Detect(image.NewRGBA(image.Rectangle{})); len(got) != 0 {
		t.Errorf("Detect = %v, erwartet keine Treffer", got)
	}
}
