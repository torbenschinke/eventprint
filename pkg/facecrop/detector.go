//go:build !nofacecrop

package facecrop

import (
	_ "embed"
	"fmt"
	"image"
	"log/slog"
	"os"
	"sync"

	"gocv.io/x/gocv"
	xdraw "golang.org/x/image/draw"
)

const (
	// detectionWidth ist die Breite, auf die ein Bild vor der Erkennung
	// verkleinert wird. 1024 px genügen für Weitwinkelaufnahmen bis hinunter
	// zu etwa 13 px Gesichtsbreite und halten die Laufzeit bei rund einer
	// Sekunde statt mehrerer Sekunden auf voller Auflösung.
	detectionWidth = 1024

	// minConfidence filtert unsichere Treffer. YuNet meldet auf Motiven ohne
	// Gesicht regelmäßig Rümpfe und Muster mit Werten bis etwa 0.3; ein zu
	// niedriger Wert würde den Ausschnitt auf solche Fehltreffer ausrichten.
	minConfidence = 0.6
)

//go:embed face_detection_yunet_2023mar.onnx
var model []byte

var shared detector

type detector struct {
	mu        sync.Mutex
	net       gocv.FaceDetectorYN
	netReady  bool
	inputSize image.Point
	warnOnce  sync.Once
}

// Detect findet Gesichter in img und liefert deren Rechtecke in den
// Koordinaten des übergebenen Bildes.
//
// Die Erkennung ist ein optionaler Komfort: Schlägt sie fehl, liefert Detect
// keine Treffer, statt den Fehler nach oben zu reichen. Der Aufrufer fällt
// dann auf den mittigen Standardausschnitt zurück und ein Ausdruck scheitert
// niemals an der Bildanalyse.
func Detect(img image.Image) []image.Rectangle {
	faces, err := shared.detect(img)
	if err != nil {
		// Nur einmal melden: Fehlt OpenCV oder das Modell, beträfe das jeden
		// weiteren Druck und würde das Log fluten.
		shared.warnOnce.Do(func() {
			slog.Warn("face detection unavailable, using centered crop", "err", err)
		})

		return nil
	}

	return faces
}

func (d *detector) detect(src image.Image) ([]image.Rectangle, error) {
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil, nil
	}

	// Die Erkennung läuft auf einer verkleinerten Kopie; scale rechnet die
	// Treffer anschließend wieder auf die volle Auflösung zurück.
	scale := 1.0
	w, h := bounds.Dx(), bounds.Dy()
	input := src

	if w > detectionWidth {
		scale = float64(w) / detectionWidth
		w = detectionWidth
		h = max(1, int(float64(bounds.Dy())/scale))
		resized := image.NewRGBA(image.Rect(0, 0, w, h))
		xdraw.ApproxBiLinear.Scale(resized, resized.Bounds(), src, bounds, xdraw.Over, nil)
		input = resized
	}

	// ImageToMatRGB liefert trotz seines Namens eine BGR-Matrix – genau das,
	// was OpenCV erwartet. Eine zusätzliche Farbkonvertierung wäre falsch.
	mat, err := gocv.ImageToMatRGB(input)
	if err != nil {
		return nil, fmt.Errorf("cannot convert face detector input: %w", err)
	}
	defer mat.Close()

	// FaceDetectorYN hält Zustand (Eingabegröße) und ist nicht threadsicher.
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureNet(image.Pt(w, h)); err != nil {
		return nil, err
	}

	result := gocv.NewMat()
	defer result.Close()

	d.net.Detect(mat, &result)
	if err := gocv.LastExceptionError(); err != nil {
		return nil, fmt.Errorf("YuNet detection failed: %w", err)
	}

	// Ergebnis ist eine Nx15-Matrix: Spalten 0..3 Rechteck, 4..13 Landmarken,
	// 14 Score.
	faces := make([]image.Rectangle, 0, result.Rows())

	for row := range result.Rows() {
		if result.GetFloatAt(row, 14) < minConfidence {
			continue
		}

		x := bounds.Min.X + int(float64(result.GetFloatAt(row, 0))*scale)
		y := bounds.Min.Y + int(float64(result.GetFloatAt(row, 1))*scale)
		fw := int(float64(result.GetFloatAt(row, 2)) * scale)
		fh := int(float64(result.GetFloatAt(row, 3)) * scale)

		// YuNet meldet bei randnahen Gesichtern Rechtecke, die über das Bild
		// hinausragen.
		if face := image.Rect(x, y, x+fw, y+fh).Intersect(bounds); !face.Empty() {
			faces = append(faces, face)
		}
	}

	return faces, nil
}

// ensureNet lädt das Netz beim ersten Aufruf und passt danach nur noch die
// Eingabegröße an. Der Aufrufer muss d.mu halten.
func (d *detector) ensureNet(size image.Point) error {
	if !d.netReady {
		path, cleanup, err := writeModel()
		if err != nil {
			return err
		}
		// OpenCV liest die Datei vollständig beim Erzeugen des Detektors ein,
		// danach wird sie nicht mehr gebraucht.
		defer cleanup()

		d.net = gocv.NewFaceDetectorYN(path, "", size)
		if err := gocv.LastExceptionError(); err != nil {
			return fmt.Errorf("cannot load YuNet model: %w", err)
		}

		// Ohne dieses Limit filtert YuNet erst ab 0.9 und übersieht damit
		// einen Teil der Gesichter in Weitwinkelaufnahmen.
		d.net.SetScoreThreshold(minConfidence)
		if err := gocv.LastExceptionError(); err != nil {
			return fmt.Errorf("cannot configure YuNet threshold: %w", err)
		}

		d.netReady = true
		d.inputSize = size
	}

	if d.inputSize != size {
		d.net.SetInputSize(size)
		if err := gocv.LastExceptionError(); err != nil {
			return fmt.Errorf("cannot resize YuNet input: %w", err)
		}

		d.inputSize = size
	}

	return nil
}

// writeModel legt das eingebettete Modell als temporäre Datei ab, weil OpenCV
// nur einen Dateipfad annimmt.
//
// Der Name wird pro Aufruf eindeutig vergeben: Ein fester Name in /tmp würde
// auf Mehrbenutzersystemen an den Rechten des Erstanlegers scheitern.
func writeModel() (path string, cleanup func(), err error) {
	file, err := os.CreateTemp("", "eventprint-yunet-*.onnx")
	if err != nil {
		return "", nil, fmt.Errorf("cannot create temporary model file: %w", err)
	}

	name := file.Name()
	cleanup = func() {
		_ = os.Remove(name)
	}

	if _, err := file.Write(model); err != nil {
		_ = file.Close()
		cleanup()

		return "", nil, fmt.Errorf("cannot write embedded YuNet model: %w", err)
	}

	if err := file.Close(); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("cannot close temporary model file: %w", err)
	}

	return name, cleanup, nil
}
