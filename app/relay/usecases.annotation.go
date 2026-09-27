package relay

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[UploadAddress](
	spec.Satisfies(upload.RUploadEingang, upload.RUploadSitzung),
	spec.Help(`Liefert die Adresse für den QR-Code; im Heimbetrieb mit dem Kennzeichen für den Upload in den Eingang.`),
)

var _ = spec.For[BeginPairing](
	spec.Satisfies(upload.RUploadKopplung),
	spec.Help(`Bittet den Upload-Dienst um einen Code an die Mailadresse des Besitzers.`),
)

var _ = spec.For[CompletePairing](
	spec.Satisfies(upload.RUploadKopplung),
	spec.Help(`Schickt den eingetippten Code und speichert bei Erfolg Adresse und Token selbst.`),
)
