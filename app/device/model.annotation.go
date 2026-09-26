package device

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

// Hell, dunkel oder nach der Tageszeit. Die Bemessung für die verschiedenen
// Panels ist Sache der Oberfläche und hat kein Gegenstück im Modell.
var _ = spec.For[Appearance](
	spec.Satisfies(modus.RModusAnzeige),
)
