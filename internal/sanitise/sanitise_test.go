package sanitise

import (
	"strings"
	"testing"
)

func TestSanitise_StripsScript(t *testing.T) {
	t.Parallel()
	in := `<p>hi</p><script>alert('xss')</script>`
	got := DefaultPolicy().Sanitise(in)
	if strings.Contains(got, "<script>") || strings.Contains(got, "alert") {
		t.Errorf("script not stripped: got %q", got)
	}
	if !strings.Contains(got, "<p>hi</p>") {
		t.Errorf("legitimate <p> stripped: got %q", got)
	}
}
