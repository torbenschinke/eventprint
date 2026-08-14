package printing_test

import (
	"testing"

	"github.com/torbenschinke/eventprint/printing"
)

// Echte Ausgabe von "lpstat -l -W completed -o CZ01" auf einem deutschen
// System. Sie enthält sowohl einen abgebrochenen als auch einen erfolgreichen
// Auftrag – genau die Unterscheidung, die vorher fehlte.
const lpstatCompletedDE = `CZ01-10                 tschinke        711680   Di 11 Aug 2026 22:23:47 CEST
	Status: The print file could not be opened.
	Alarme: canceled-at-device
	in Warteschlange eingereiht für CZ01
CZ01-9                  tschinke        758784   Di 11 Aug 2026 22:23:37 CEST
	Status: 
	Alarme: job-completed-successfully
	in Warteschlange eingereiht für CZ01
`

// Dieselbe Ausgabe auf einem englischen System. Die Beschriftungen ändern
// sich, die IPP-Kennungen nicht – deshalb wertet der Parser nur die Werte aus.
const lpstatCompletedEN = `CZ01-10                 tschinke        711680   Tue 11 Aug 2026 10:23:47 PM CEST
	Status: The print file could not be opened.
	Alerts: canceled-at-device
	queued for CZ01
CZ01-9                  tschinke        758784   Tue 11 Aug 2026 10:23:37 PM CEST
	Status: 
	Alerts: job-completed-successfully
	queued for CZ01
`

func TestParseJobStatus(t *testing.T) {
	tests := []struct {
		name        string
		out         string
		jobID       string
		wantDone    bool
		wantSuccess bool
		wantReason  string
		wantMessage string
	}{
		{
			name:        "abgebrochener Auftrag, deutsch",
			out:         lpstatCompletedDE,
			jobID:       "CZ01-10",
			wantDone:    true,
			wantSuccess: false,
			wantReason:  "canceled-at-device",
			wantMessage: "The print file could not be opened.",
		},
		{
			name:        "erfolgreicher Auftrag, deutsch",
			out:         lpstatCompletedDE,
			jobID:       "CZ01-9",
			wantDone:    true,
			wantSuccess: true,
			wantReason:  "job-completed-successfully",
		},
		{
			name:        "abgebrochener Auftrag, englisch",
			out:         lpstatCompletedEN,
			jobID:       "CZ01-10",
			wantDone:    true,
			wantSuccess: false,
			wantReason:  "canceled-at-device",
			wantMessage: "The print file could not be opened.",
		},
		{
			name:        "erfolgreicher Auftrag, englisch",
			out:         lpstatCompletedEN,
			jobID:       "CZ01-9",
			wantDone:    true,
			wantSuccess: true,
			wantReason:  "job-completed-successfully",
		},
		{
			name:  "noch laufender Auftrag steht nicht in der Liste",
			out:   lpstatCompletedDE,
			jobID: "CZ01-11",
		},
		{
			name:  "leere Ausgabe",
			out:   "",
			jobID: "CZ01-1",
		},
		{
			// Ohne genaue Zuordnung würde CZ01-1 den Block von CZ01-10
			// erben und ein Fehler dem falschen Auftrag zugeschrieben.
			name:  "Kennung ist kein Präfix einer anderen",
			out:   lpstatCompletedDE,
			jobID: "CZ01-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := printing.ParseJobStatus(tt.out, tt.jobID)

			if got.Done != tt.wantDone {
				t.Errorf("Done = %v, erwartet %v", got.Done, tt.wantDone)
			}

			if got.Success != tt.wantSuccess {
				t.Errorf("Success = %v, erwartet %v", got.Success, tt.wantSuccess)
			}

			if got.Reason != tt.wantReason {
				t.Errorf("Reason = %q, erwartet %q", got.Reason, tt.wantReason)
			}

			if got.Message != tt.wantMessage {
				t.Errorf("Message = %q, erwartet %q", got.Message, tt.wantMessage)
			}
		})
	}
}

// TestParseRequestID deckt die übersetzte Bestätigung von lp ab.
func TestParseRequestID(t *testing.T) {
	tests := []struct {
		name  string
		out   string
		queue string
		want  string
	}{
		{
			name:  "deutsch",
			out:   "Anfrage-ID ist CZ01-7 (1 Datei(en))",
			queue: "CZ01",
			want:  "CZ01-7",
		},
		{
			name:  "englisch",
			out:   "request id is CZ01-7 (1 file(s))",
			queue: "CZ01",
			want:  "CZ01-7",
		},
		{
			name:  "Warteschlange mit Bindestrich im Namen",
			out:   "request id is Foto-Drucker-42 (1 file(s))",
			queue: "Foto-Drucker",
			want:  "Foto-Drucker-42",
		},
		{
			name:  "andere Warteschlange wird nicht verwechselt",
			out:   "request id is Andere-7 (1 file(s))",
			queue: "CZ01",
			want:  "",
		},
		{
			name:  "keine Ausgabe",
			out:   "",
			queue: "CZ01",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printing.ParseRequestID(tt.out, tt.queue); got != tt.want {
				t.Errorf("ParseRequestID = %q, erwartet %q", got, tt.want)
			}
		})
	}
}

// TestPrinterStatusProblem prüft die Sätze, die der Bedienung auf der
// Druckstatus-Seite angezeigt werden.
func TestPrinterStatusProblem(t *testing.T) {
	tests := []struct {
		name    string
		status  printing.PrinterStatus
		wantOK  bool
		wantHas bool
	}{
		{
			name:    "alles bereit",
			status:  printing.PrinterStatus{Queue: "CZ01", Exists: true, Enabled: true, Accepting: true},
			wantOK:  true,
			wantHas: false,
		},
		{
			name:    "Warteschlange fehlt",
			status:  printing.PrinterStatus{Queue: "CZ01"},
			wantHas: true,
		},
		{
			name:    "Drucker angehalten",
			status:  printing.PrinterStatus{Queue: "CZ01", Exists: true, Accepting: true},
			wantHas: true,
		},
		{
			name:    "nimmt keine Aufträge an",
			status:  printing.PrinterStatus{Queue: "CZ01", Exists: true, Enabled: true},
			wantHas: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.OK(); got != tt.wantOK {
				t.Errorf("OK = %v, erwartet %v", got, tt.wantOK)
			}

			if got := tt.status.Problem() != ""; got != tt.wantHas {
				t.Errorf("Problem() = %q", tt.status.Problem())
			}
		})
	}
}
