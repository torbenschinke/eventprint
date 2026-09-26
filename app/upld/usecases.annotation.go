package upld

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/quellen"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[OpenSession](
	spec.Satisfies(upload.RUploadSitzung),
	spec.Help(`Legt eine neue, kurzlebige Upload-Identität für die anfragende
Fotobox an. Eine bestehende Sitzung derselben Fotobox wird dabei verworfen.`),
)

var _ = spec.For[FindPendingJobs](
	spec.Satisfies(upload.RUploadAbholung),
	spec.Help(`Liefert die Aufträge, die für die anfragende Fotobox bereitliegen.`),
)

var _ = spec.For[OpenJobImage](
	spec.Satisfies(upload.RUploadBild),
	spec.Help(`Öffnet das Originalbild eines wartenden Auftrags.`),
)

var _ = spec.For[AckJob](
	spec.Satisfies(upload.RUploadBestaetigung),
	spec.Help(`Bestätigt einen übernommenen Auftrag und entfernt ihn samt Bild.`),
)

var _ = spec.For[RegisterLogin](
	spec.Satisfies(quellen.RQuellenLightroomAnmeldung),
	spec.Help(`Merkt sich für die anfragende Fotobox eine begonnene Anmeldung bei
Adobe und liefert den kurzen Link, den die Fotobox als QR-Code zeigt.`),
)

var _ = spec.For[CollectLogin](
	spec.Satisfies(quellen.RQuellenLightroomAnmeldung),
	spec.Help(`Gibt der Fotobox, die die Anmeldung begonnen hat, den Code von Adobe
genau einmal heraus.`),
)
