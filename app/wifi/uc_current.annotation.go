package wifi

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/netz"
)

var _ = spec.For[Current](
	spec.Satisfies(netz.RNetzZustand, netz.RNetzBetreuung),
	spec.Help(`Liefert die bestehende Funkverbindung mit Netzname und Feldstärke.
Sucht dabei nicht neu, damit der Zustand sofort dasteht. Die Feldstärke stammt
aus der Netzliste, weil sie in der Geräteliste nicht enthalten ist.`),
)
