package printing

import (
	"image"
	"testing"
)

// polaroidArea entspricht dem Motivfenster, das renderTemplate für das
// Polaroid aufspannt (1056x1464). Die Tests rechnen bewusst gegen dieselbe
// Geometrie, die auch gedruckt wird.
var polaroidArea = image.Rect(0, 0, 1056, 1464)

func TestCropForFacesCentersGroupInUpperHalf(t *testing.T) {
	bounds := image.Rect(0, 0, 4000, 3000)
	group := image.Rect(2000, 1200, 2700, 1600)
	crop := cropForFaces(bounds, []image.Rectangle{group}, polaroidArea)

	if crop.Empty() || !crop.In(bounds) {
		t.Fatalf("crop %v liegt nicht im Original %v", crop, bounds)
	}
	wantX := group.Min.X + group.Dx()/2
	wantY := group.Min.Y + group.Dy()/2
	if got := crop.Min.X + crop.Dx()/2; absInt(got-wantX) > 1 {
		t.Errorf("Gesichtsmitte horizontal bei %d, erwartet %d", got, wantX)
	}
	if got := crop.Min.Y + crop.Dy()/4; absInt(got-wantY) > 1 {
		t.Errorf("Gesichtsmitte vertikal bei %d, erwartet %d", got, wantY)
	}
}

// Der Ausschnitt wird unverzerrt auf das Motivfenster abgebildet, deshalb
// muss sein Seitenverhältnis exakt passen.
func TestCropForFacesKeepsAreaAspect(t *testing.T) {
	bounds := image.Rect(0, 0, 6000, 4000)
	crop := cropForFaces(bounds, []image.Rectangle{image.Rect(2600, 900, 3400, 1500)}, polaroidArea)

	want := float64(polaroidArea.Dx()) / float64(polaroidArea.Dy())
	got := float64(crop.Dx()) / float64(crop.Dy())
	if diff := got - want; diff > 0.002 || diff < -0.002 {
		t.Errorf("Seitenverhältnis = %.4f, erwartet %.4f (crop %v)", got, want, crop)
	}
}

// Padding: links, rechts und oben bleiben 10 % der Ausschnitthöhe frei,
// sofern das Bild groß genug ist.
func TestCropForFacesKeepsPadding(t *testing.T) {
	bounds := image.Rect(0, 0, 6000, 6000)
	group := image.Rect(2600, 2000, 3400, 2400)
	crop := cropForFaces(bounds, []image.Rectangle{group}, polaroidArea)

	pad := int(0.10 * float64(crop.Dy()))
	if group.Min.X-crop.Min.X < pad-1 {
		t.Errorf("linkes Padding %d < %d", group.Min.X-crop.Min.X, pad)
	}
	if crop.Max.X-group.Max.X < pad-1 {
		t.Errorf("rechtes Padding %d < %d", crop.Max.X-group.Max.X, pad)
	}
	if group.Min.Y-crop.Min.Y < pad-1 {
		t.Errorf("oberes Padding %d < %d", group.Min.Y-crop.Min.Y, pad)
	}
}

func TestCropForFacesIncludesAllFaces(t *testing.T) {
	bounds := image.Rect(0, 0, 6000, 6000)
	faces := []image.Rectangle{
		image.Rect(1200, 900, 1600, 1400),
		image.Rect(3600, 1000, 4100, 1500),
	}
	crop := cropForFaces(bounds, faces, polaroidArea)

	for _, face := range faces {
		if face.Intersect(crop) != face {
			t.Errorf("crop %v schneidet Gesicht %v an", crop, face)
		}
	}
}

// Ein winziges Gesicht darf nicht in einen Ausschnitt zoomen, der kleiner
// als das Motivfenster ist – sonst wird der Ausdruck hochskaliert und unscharf.
func TestCropForFacesNeverUpscales(t *testing.T) {
	bounds := image.Rect(0, 0, 6000, 4000)
	crop := cropForFaces(bounds, []image.Rectangle{image.Rect(3000, 1000, 3060, 1080)}, polaroidArea)

	if crop.Dx() < polaroidArea.Dx() || crop.Dy() < polaroidArea.Dy() {
		t.Errorf("crop %v ist kleiner als das Motivfenster %v", crop, polaroidArea)
	}
}

func TestCropForFacesClampsAtImageEdge(t *testing.T) {
	bounds := image.Rect(0, 0, 1600, 2400)
	crop := cropForFaces(bounds, []image.Rectangle{image.Rect(0, 0, 500, 300)}, polaroidArea)

	if crop.Empty() || !crop.In(bounds) {
		t.Fatalf("crop %v liegt nicht im Original %v", crop, bounds)
	}
	if crop.Min.X != bounds.Min.X || crop.Min.Y != bounds.Min.Y {
		t.Errorf("randnaher crop = %v, erwartet an oberer linker Kante", crop)
	}
}

// Ist das Original kleiner als das Motivfenster, gewinnt die Bildgrenze.
func TestCropForFacesNeverExceedsSource(t *testing.T) {
	bounds := image.Rect(0, 0, 300, 500)
	crop := cropForFaces(bounds, []image.Rectangle{image.Rect(100, 100, 180, 200)}, polaroidArea)

	if !crop.In(bounds) {
		t.Errorf("crop %v verlässt das Original %v", crop, bounds)
	}
}

func TestCropForFacesWithoutDetectionFallsBack(t *testing.T) {
	if got := cropForFaces(image.Rect(0, 0, 1000, 1000), nil, polaroidArea); !got.Empty() {
		t.Errorf("crop ohne Gesichter = %v, erwartet leer", got)
	}
}

// Treffer außerhalb des Bildes dürfen die Gruppenbox nicht aufblähen.
func TestFaceGroupClipsToBounds(t *testing.T) {
	bounds := image.Rect(0, 0, 1000, 1000)
	got := faceGroup(bounds, []image.Rectangle{
		image.Rect(-50, -50, 100, 100),
		image.Rect(2000, 2000, 2100, 2100),
	})

	if want := image.Rect(0, 0, 100, 100); got != want {
		t.Errorf("faceGroup = %v, erwartet %v", got, want)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
