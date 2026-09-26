package photoupld

import "testing"

func TestUploadURL(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		fallback string
		want     string
	}{
		{name: "configured", settings: Settings{PublicURL: "https://upload.example.de/base/"}, want: "https://upload.example.de/base/upload?u=abc"},
		{name: "adds scheme", settings: Settings{PublicURL: "upload.example.de"}, want: "https://upload.example.de/upload?u=abc"},
		{name: "fallback", fallback: "http://localhost:3000", want: "http://localhost:3000/upload?u=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.settings.UploadURL("abc", func() string { return tt.fallback })
			if got != tt.want {
				t.Fatalf("UploadURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOAuthURLs(t *testing.T) {
	configured := Settings{PublicURL: "https://upload.example.de/base/"}
	fallback := func() string { return "http://localhost:3000/" }

	if got, want := configured.OAuthStartURL("abc_-1", fallback), "https://upload.example.de/base/oauth/adobe/start?s=abc_-1"; got != want {
		t.Fatalf("OAuthStartURL = %q, want %q", got, want)
	}

	if got, want := configured.OAuthCallbackURL(fallback), "https://upload.example.de/base/oauth/adobe/callback"; got != want {
		t.Fatalf("OAuthCallbackURL = %q, want %q", got, want)
	}

	if got, want := (Settings{}).OAuthCallbackURL(fallback), "http://localhost:3000/oauth/adobe/callback"; got != want {
		t.Fatalf("OAuthCallbackURL (fallback) = %q, want %q", got, want)
	}
}
