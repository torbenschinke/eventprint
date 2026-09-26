package nas

// Die Fehler sind so formuliert, dass die Oberfläche sie zeigen kann: Wer an
// der Box steht, soll wissen, was er ändern muss, nicht welcher NTSTATUS kam.
const (
	ErrNotConfigured Error = "Kein NAS eingerichtet. Trage es unter Einstellungen → Konten & Quellen ein."
	ErrUnreachable   Error = "Das NAS antwortet nicht. Ist es eingeschaltet und im selben Netz?"
	ErrLogin         Error = "Benutzername oder Kennwort stimmen nicht."
	ErrShare         Error = "Diese Freigabe gibt es nicht, oder der Benutzer darf sie nicht öffnen."
	ErrDenied        Error = "Der Benutzer darf diesen Ordner nicht lesen."
	ErrNoImage       Error = "Das ist kein Bild, das die Box drucken kann."
	ErrTooLarge      Error = "Das Bild ist zu groß für die Box."
)

// Error ist ein Fehler mit einer Meldung für Menschen. Als Konstante lässt er
// sich mit errors.Is prüfen, ohne dass die Anwendungsfälle Paketzustand lesen.
type Error string

func (e Error) Error() string { return string(e) }

// failure trägt eine Meldung für Menschen und die Ursache für das Protokoll.
// errors.Is findet beide.
type failure struct {
	msg   error
	cause error
}

func (f failure) Error() string   { return f.msg.Error() }
func (f failure) Unwrap() []error { return []error{f.msg, f.cause} }
