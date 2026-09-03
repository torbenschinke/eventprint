package cfgphotobox

import "testing"

// TestPublicURLFor deckt die Fälle ab, die hinter einem Reverse Proxy
// tatsächlich auftreten. Steht hier eine falsche Adresse, ist der QR-Code auf
// dem Fotobox-Display wertlos.
func TestPublicURLFor(t *testing.T) {
	fallback := func() string { return "http://127.0.0.1:3000/upload" }

	tests := []struct {
		name     string
		settings Settings
		want     string
	}{
		{
			name:     "ohne Einstellung greift die automatische Ermittlung",
			settings: Settings{},
			want:     "http://127.0.0.1:3000/upload",
		},
		{
			name:     "nur Leerzeichen zählt als leer",
			settings: Settings{PublicURL: "   "},
			want:     "http://127.0.0.1:3000/upload",
		},
		{
			name:     "vollständige Adresse",
			settings: Settings{PublicURL: "https://fotobox.example.de"},
			want:     "https://fotobox.example.de/upload",
		},
		{
			name:     "abschließender Schrägstrich erzeugt keinen doppelten",
			settings: Settings{PublicURL: "https://fotobox.example.de/"},
			want:     "https://fotobox.example.de/upload",
		},
		{
			name:     "Unterpfad des Reverse Proxy bleibt erhalten",
			settings: Settings{PublicURL: "https://example.de/fotobox"},
			want:     "https://example.de/fotobox/upload",
		},
		{
			name:     "Port bleibt erhalten",
			settings: Settings{PublicURL: "http://192.168.20.26:3000"},
			want:     "http://192.168.20.26:3000/upload",
		},
		{
			name:     "fehlendes Schema wird ergänzt, sonst wäre der QR-Code unbrauchbar",
			settings: Settings{PublicURL: "fotobox.example.de"},
			want:     "https://fotobox.example.de/upload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.settings.PublicURLFor("upload", fallback); got != tt.want {
				t.Errorf("PublicURLFor = %q, erwartet %q", got, tt.want)
			}
		})
	}
}
