package uiphotobox

import (
	"go.wdy.de/nago/application/image"
	httpimage "go.wdy.de/nago/application/image/http"
	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"

	"github.com/torbenschinke/eventprint/photo"
)

// photoTile rendert ein Foto als anklickbare Kachel.
//
// Die Kachel ist quadratisch und schneidet das Motiv per FitCover zu. Das
// hält das Raster auch bei gemischten Hoch- und Querformaten ruhig – auf
// einer Feier landen Kamerabilder und Handybilder nebeneinander.
func photoTile(p photo.Photo, size ui.Length, onClick func()) core.View {
	return ui.VStack(
		ui.Image().
			URI(httpimage.URI(p.Image, image.FitCover, thumbnailPx, thumbnailPx)).
			ObjectFit(ui.FitCover).
			AccessibilityLabel(p.String()).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full)),
	).
		BackgroundColor(ui.M3).
		Action(onClick).
		Border(ui.Border{}.Radius(ui.L12)).
		Frame(ui.Frame{}.Size(size, size))
}

// thumbnailPx ist die angeforderte Kantenlänge der Vorschau. Das
// Image-Subsystem liefert daraufhin die kleinste passende Variante, statt das
// mehrere Megabyte große Original über die Leitung zu schicken.
const thumbnailPx = 512

// emptyHint erscheint, solange noch kein einziges Foto vorhanden ist.
func emptyHint(title, message string) core.View {
	return ui.VStack(
		ui.Text(title).Font(ui.TitleMedium),
		ui.Text(message).Font(ui.BodyMedium).
			TextAlignment(ui.TextAlignCenter),
	).
		Gap(ui.L8).
		Alignment(ui.Center).
		WithPadding(ui.Padding{}.All(ui.L40)).
		Frame(ui.Frame{}.FullWidth())
}
