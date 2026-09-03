package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

var _ = spec.For[Delete](
	spec.Satisfies(foto.RFotoLoeschen),
	spec.Help(`Entfernt ein Bild aus der Historie.`),
)
