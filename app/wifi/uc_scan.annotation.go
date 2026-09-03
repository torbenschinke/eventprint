package wifi

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/netz"
)

var _ = spec.For[Scan](
	spec.Satisfies(netz.RNetzSuche, netz.RNetzBetreuung),
	spec.Help(`Sucht nach empfangbaren Funknetzen und fasst sie nach Namen zusammen.
Ein Funknetz erscheint einmal je Zugangspunkt und Frequenzband; die Liste
zeigt es einmal, mit der besten Feldstärke. Der Aufruf dauert Sekunden, weil
das Funkgerät dafür die Kanäle abhört.`),
)
