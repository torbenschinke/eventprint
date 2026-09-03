package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

var _ = spec.For[OpenOriginal](
	spec.Satisfies(foto.RFotoDruckvorlage),
	spec.Help(`Öffnet die unveränderten Originaldaten als Vorlage für den Druck.`),
)
