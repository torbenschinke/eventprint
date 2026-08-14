package photo

// Imported wird veröffentlicht, sobald ein neues Foto vollständig
// eingespielt und persistiert wurde. Die Fotobox-Oberfläche kann darauf
// reagieren, um den Startbildschirm zu aktualisieren.
type Imported struct {
	Photo  ID
	Source Source
}

// Deleted wird veröffentlicht, nachdem ein Foto entfernt wurde.
type Deleted struct {
	Photo ID
}
