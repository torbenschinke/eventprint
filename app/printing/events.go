package printing

// JobQueued wird veröffentlicht, sobald ein Auftrag in der Warteschlange
// liegt.
type JobQueued struct {
	Job JobID
}

// JobFinished wird veröffentlicht, sobald ein Auftrag abgeschlossen ist –
// erfolgreich oder nicht.
type JobFinished struct {
	Job     JobID
	State   State
	Message string
}
