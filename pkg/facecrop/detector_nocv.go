//go:build nofacecrop

package facecrop

import (
	"image"
	"log/slog"
	"sync"
)

// Diese Fassung wird mit dem Build-Tag "nofacecrop" übersetzt und kommt ohne
// OpenCV aus.
//
// Sie existiert für Maschinen, auf denen sich gocv nicht gegen die vorhandene
// OpenCV-Fassung übersetzen lässt – auf einem Raspberry Pi ist die Version aus
// der Distribution regelmäßig zu alt. Die Fotobox soll dort trotzdem laufen:
// Ohne erkannte Gesichter greift überall der mittige Standardausschnitt, was
// genau das Verhalten ist, das auch die Gesichtserkennung ohne Treffer zeigt.

var warnOnce sync.Once

// Detect meldet keine Gesichter.
func Detect(image.Image) []image.Rectangle {
	warnOnce.Do(func() {
		slog.Warn("face detection is not compiled in",
			"tag", "nofacecrop",
			"effect", "Der automatische Bildausschnitt verwendet durchgehend den mittigen Standardausschnitt.",
		)
	})

	return nil
}
