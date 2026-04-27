package feedparse

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinutes_NonCJK_WordCount(t *testing.T) {
	// 600 simple English words → 600/200 = 3 min.
	body := strings.Repeat("hello ", 600)
	require.Equal(t, 3, Minutes(body))
}

func TestMinutes_CJK_CharCount(t *testing.T) {
	// 600 Chinese characters → 600/200 = 3 min.
	body := strings.Repeat("文", 600)
	require.Equal(t, 3, Minutes(body))
}

func TestMinutes_CJKDetectedFromFirstChars(t *testing.T) {
	// Mostly Latin but starts with CJK → CJK rule wins.
	// (Spec: scan first 50 chars.)
	body := "日本語" + strings.Repeat(" word", 1000)
	got := Minutes(body)
	require.Less(t, got, 5,
		"CJK rule (chars/200) must beat the inflated Latin word count")
}

func TestMinutes_HTMLStripped(t *testing.T) {
	body := strings.Repeat("<p>hello</p> ", 600)
	got := Minutes(body)
	// 600 "hello" tokens → 3 min. Tags must not inflate.
	require.Equal(t, 3, got)
}

func TestMinutes_AtLeastOne(t *testing.T) {
	require.Equal(t, 1, Minutes("short"))
	require.Equal(t, 1, Minutes(""))
}
