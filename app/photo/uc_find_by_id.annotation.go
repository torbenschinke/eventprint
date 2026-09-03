package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

var _ = spec.For[FindByID](
	spec.Satisfies(foto.RFotoEinzelbild),
	spec.Help(`Liefert ein einzelnes Bild anhand seiner Kennung.`),
)
