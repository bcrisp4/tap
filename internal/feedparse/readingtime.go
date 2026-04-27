package feedparse

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

const (
	wordsPerMinute = 200
	charsPerMinute = 200
	cjkProbeChars  = 50
)

// Minutes returns the estimated reading time in minutes for an HTML
// or plain-text body. Always at least 1.
func Minutes(htmlOrText string) int {
	text := stripTags(htmlOrText)
	if text == "" {
		return 1
	}

	if isCJK(text, cjkProbeChars) {
		// Count CJK characters only — mixing in Latin would inflate the
		// estimate when feeds embed code blocks, English captions, etc.
		// (Plain Latin text is handled by the word-count branch below.)
		n := 0
		for _, r := range text {
			if isCJKRune(r) {
				n++
			}
		}
		return atLeastOne(n / charsPerMinute)
	}

	// Word count.
	n := len(strings.Fields(text))
	return atLeastOne(n / wordsPerMinute)
}

// stripTags removes HTML markup, keeping text nodes only.
func stripTags(s string) string {
	if !strings.ContainsAny(s, "<>") {
		return s
	}
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return b.String()
		case html.TextToken:
			b.Write(z.Text())
			b.WriteByte(' ')
		}
	}
}

// isCJK reports whether the first probe runes contain a CJK script.
// Spec §4: Han, Hiragana, Katakana, or Hangul.
func isCJK(s string, probe int) bool {
	count := 0
	for _, r := range s {
		if count >= probe {
			break
		}
		if unicode.IsSpace(r) {
			continue
		}
		count++
		if isCJKRune(r) {
			return true
		}
	}
	return false
}

// isCJKRune reports whether r belongs to one of the CJK scripts the
// reading-time heuristic accounts for: Han, Hiragana, Katakana, Hangul.
func isCJKRune(r rune) bool {
	switch {
	case unicode.Is(unicode.Han, r),
		unicode.Is(unicode.Hiragana, r),
		unicode.Is(unicode.Katakana, r),
		unicode.Is(unicode.Hangul, r):
		return true
	}
	return false
}

func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
