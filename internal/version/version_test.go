package version

import "testing"

func TestStringDefault(t *testing.T) {
	if got, want := String(), "0.0.0-dev"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
