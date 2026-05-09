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

func TestSanitise_DropsDangerousSchemes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
	}{
		{"javascript in href", `<a href="javascript:alert(1)">click</a>`},
		{"javascript in img src", `<img src="javascript:alert(1)">`},
		{"data in img src", `<img src="data:image/png;base64,AAAA">`},
		{"vbscript in href", `<a href="vbscript:msgbox(1)">click</a>`},
	}
	p := DefaultPolicy()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			for _, scheme := range []string{"javascript:", "data:", "vbscript:"} {
				if strings.Contains(got, scheme) {
					t.Errorf("scheme %q survived in %q output: %q", scheme, tc.name, got)
				}
			}
		})
	}
}
