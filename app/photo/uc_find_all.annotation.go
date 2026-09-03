package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

var _ = spec.For[FindAll](
	spec.Satisfies(foto.RFotoHistorie),
	spec.Help(`Liefert alle Bilder, beginnend mit dem neuesten.`),
)
