package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
)

var _ = spec.For[InspectArchive](
	spec.Satisfies(archiv.RArchivPlatz),
	spec.Help(`Meldet Anzahl und Platzbedarf der archivierten Bilder.
Halb geschriebene Dateien eines laufenden Imports zählen nicht mit, sonst
täuschte die Anzeige Bestand vor.`),
)
