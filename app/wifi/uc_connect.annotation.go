package wifi

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/netz"
)

var _ = spec.For[Connect](
	spec.Satisfies(netz.RNetzVerbinden, netz.RNetzBetreuung),
	spec.Help(`Verbindet die Fotobox mit einem Funknetz.
Ein leeres Kennwort bedeutet ein offenes Netz. Ein abgelehntes Kennwort wird
als WrongPasswordError gemeldet und ist damit vom sonstigen Scheitern zu
unterscheiden: Das eine heißt neu eintippen, das andere näher ans Funknetz.`),
)
