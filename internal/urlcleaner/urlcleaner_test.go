package urlcleaner

import "testing"

func TestClean_StripsUtmSource(t *testing.T) {
	t.Parallel()
	got := Clean("https://example.com/page?utm_source=newsletter&id=42")
	want := "https://example.com/page?id=42"
	if got != want {
		t.Errorf("Clean() = %q, want %q", got, want)
	}
}
