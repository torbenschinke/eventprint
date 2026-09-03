package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
)

var _ = spec.For[PurgeEvent](
	spec.Satisfies(archiv.RArchivLoeschen),
	spec.Help(`Räumt die Fotobox für die nächste Veranstaltung frei.
Entfernt Fotos, Bilddaten und Archivdateien endgültig. Anders als Delete, das
bewusst nur die Metadaten entfernt und die Bilddaten für den Fall eines
Versehens liegen lässt, gibt erst dieser Anwendungsfall den Speicher frei.
Die Bilddaten werden vor dem Eintrag gelöscht: Andersherum entstünde ein Blob,
den niemand mehr findet.`),
)
