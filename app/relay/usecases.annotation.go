package relay

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[UploadAddress](
	spec.Satisfies(upload.RUploadEingang, upload.RUploadSitzung),
	spec.Help(`Liefert die Adresse für den QR-Code; im Heimbetrieb mit dem Kennzeichen für den Upload in den Eingang.`),
)
