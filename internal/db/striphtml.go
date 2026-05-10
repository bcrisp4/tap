package db

import (
	"bytes"
	"database/sql/driver"
	"strings"

	"modernc.org/sqlite"
	"golang.org/x/net/html"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("tap_strip_html", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			s, _ := args[0].(string)
			return stripHTML(s), nil
		},
	)
}

// stripHTML returns the plain-text content of an HTML fragment.
// Walks the parse tree and concatenates text nodes separated by spaces.
func stripHTML(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				if buf.Len() > 0 {
					buf.WriteByte(' ')
				}
				buf.WriteString(t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return buf.String()
}
