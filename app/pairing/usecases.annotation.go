package pairing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[RequestPairing](
	spec.Satisfies(upload.RUploadKopplung),
	spec.Help(`Beginnt die Kopplung; verschickt einen Code nur an registrierte, bestätigte Nutzer und antwortet in jedem Fall gleich.`),
)

var _ = spec.For[ConfirmPairing](
	spec.Satisfies(upload.RUploadKopplung),
	spec.Help(`Prüft den Code, sperrt nach fünf Fehlversuchen und stellt bei Erfolg das Token der Box aus.`),
)
