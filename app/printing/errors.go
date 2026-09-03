package printing

import "go.wdy.de/nago/pkg/std"

// errPhotoGone tritt auf, wenn ein Foto zwischen dem Anlegen und dem
// Ausführen des Druckauftrags gelöscht wurde.
var errPhotoGone = std.NewLocalizedError(
	"Foto nicht gefunden",
	"Das Foto wurde gelöscht, bevor es gedruckt werden konnte.",
)
