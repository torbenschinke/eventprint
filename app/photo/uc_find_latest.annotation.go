package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

var _ = spec.For[FindLatest](
	spec.Satisfies(foto.RFotoHistorie),
	spec.Help(`Liefert die jüngsten Bilder für den Startbildschirm.`),
)
