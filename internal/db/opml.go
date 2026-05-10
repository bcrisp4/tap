package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// opmlDoc is the OPML 2.0 root element.
type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Type     string        `xml:"type,attr,omitempty"`
	Text     string        `xml:"text,attr"`
	Title    string        `xml:"title,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Children []opmlOutline `xml:"outline"`
}

// ExportOPML generates OPML 2.0 XML for a user's subscriptions grouped by category.
func ExportOPML(ctx context.Context, d *sql.DB, userID int64) ([]byte, error) {
	cats, err := ListCategories(ctx, d, userID)
	if err != nil {
		return nil, fmt.Errorf("export opml: list categories: %w", err)
	}

	subs, err := ListSubscriptions(ctx, d, userID)
	if err != nil {
		return nil, fmt.Errorf("export opml: list subscriptions: %w", err)
	}

	// Group subscriptions by category ID.
	byCat := map[int64][]Subscription{}
	var uncategorised []Subscription
	for _, s := range subs {
		if s.CategoryID.Valid {
			byCat[s.CategoryID.Int64] = append(byCat[s.CategoryID.Int64], s)
		} else {
			uncategorised = append(uncategorised, s)
		}
	}

	subToOutline := func(s Subscription) opmlOutline {
		return opmlOutline{
			Type:    "rss",
			Text:    s.Title,
			Title:   s.Title,
			XMLURL:  s.FeedURL,
			HTMLURL: s.SiteURL.String,
		}
	}

	var bodyOutlines []opmlOutline
	for _, cat := range cats {
		catSubs := byCat[cat.ID]
		children := make([]opmlOutline, 0, len(catSubs))
		for _, s := range catSubs {
			children = append(children, subToOutline(s))
		}
		bodyOutlines = append(bodyOutlines, opmlOutline{
			Type:     "folder",
			Text:     cat.Name,
			Children: children,
		})
	}
	for _, s := range uncategorised {
		bodyOutlines = append(bodyOutlines, subToOutline(s))
	}

	doc := opmlDoc{
		Version: "2.0",
		Head:    opmlHead{Title: "Tap subscriptions"},
		Body:    opmlBody{Outlines: bodyOutlines},
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("export opml: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// ImportOPML imports an OPML document for a user. It is transactional: all
// category upserts and non-erroring subscription inserts commit together, or
// all roll back on a fatal parse error.
func ImportOPML(ctx context.Context, d *sql.DB, userID int64, data []byte, now int64) (imported, skipped int, errs []string, err error) {
	var doc opmlDoc
	if xmlErr := xml.Unmarshal(data, &doc); xmlErr != nil {
		return 0, 0, nil, fmt.Errorf("parse OPML: %w", xmlErr)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	imported, skipped, errs, err = importOPMLTx(ctx, tx, userID, doc.Body.Outlines, now)
	if err != nil {
		return 0, 0, nil, err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, nil, fmt.Errorf("commit: %w", err)
	}
	return imported, skipped, errs, nil
}

func importOPMLTx(ctx context.Context, tx *sql.Tx, userID int64, outlines []opmlOutline, now int64) (imported, skipped int, errs []string, err error) {
	for _, o := range outlines {
		if isFolder(o) {
			// Upsert the category.
			catID, catErr := upsertCategory(ctx, tx, userID, o.Text, now)
			if catErr != nil {
				errs = append(errs, fmt.Sprintf("skipped folder '%s': %v", o.Text, catErr))
				continue
			}
			// Import children (flatten deeper nesting to this category).
			for _, child := range flattenOutlines(o.Children) {
				imp, skip, childErrs := importFeedOutline(ctx, tx, userID, child, &catID, now)
				imported += imp
				skipped += skip
				errs = append(errs, childErrs...)
			}
		} else if isFeed(o) {
			imp, skip, feedErrs := importFeedOutline(ctx, tx, userID, o, nil, now)
			imported += imp
			skipped += skip
			errs = append(errs, feedErrs...)
		}
	}
	return
}

// flattenOutlines collapses any nested folders to their leaf feed outlines.
func flattenOutlines(outlines []opmlOutline) []opmlOutline {
	var result []opmlOutline
	for _, o := range outlines {
		if isFeed(o) {
			result = append(result, o)
		} else if isFolder(o) {
			result = append(result, flattenOutlines(o.Children)...)
		}
	}
	return result
}

func importFeedOutline(ctx context.Context, tx *sql.Tx, userID int64, o opmlOutline, catID *int64, now int64) (imported, skipped int, errs []string) {
	feedURL := strings.TrimSpace(o.XMLURL)
	if feedURL == "" {
		feedURL = strings.TrimSpace(o.Text)
	}
	u, parseErr := url.Parse(feedURL)
	if parseErr != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
		errs = append(errs, fmt.Sprintf("skipped '%s': invalid URL", feedURL))
		return
	}

	title := strings.TrimSpace(o.Title)
	if title == "" {
		title = strings.TrimSpace(o.Text)
	}
	if title == "" {
		title = feedURL
	}

	siteURL := strings.TrimSpace(o.HTMLURL)

	_, insertErr := insertSubscriptionTx(ctx, tx, NewSubscription{
		UserID:  userID,
		Title:   title,
		FeedURL: feedURL,
		SiteURL: siteURL,
		Created: now,
	}, catID)

	if insertErr != nil {
		if errors.Is(insertErr, ErrSubscriptionExists) {
			skipped++
		} else {
			errs = append(errs, fmt.Sprintf("skipped '%s': %v", feedURL, insertErr))
		}
		return
	}
	imported++
	return
}

func insertSubscriptionTx(ctx context.Context, tx *sql.Tx, s NewSubscription, catID *int64) (int64, error) {
	var catVal interface{}
	if catID != nil {
		catVal = *catID
	}
	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO subscriptions
		    (user_id, title, feed_url, site_url, next_poll_at, created_at, extract,
		     cookie, basic_auth_user, basic_auth_pass, category_id)
		VALUES (?, ?, ?, NULLIF(?, ''), 0, ?, 0, '', '', '', ?)
	`, s.UserID, s.Title, s.FeedURL, s.SiteURL, s.Created, catVal)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, ErrSubscriptionExists
		}
		return 0, fmt.Errorf("insert subscription: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, ErrSubscriptionExists
	}
	return res.LastInsertId()
}

func upsertCategory(ctx context.Context, tx *sql.Tx, userID int64, name string, now int64) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("empty category name")
	}
	_, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO categories (user_id, name, created_at) VALUES (?, ?, ?)`,
		userID, name, now)
	if err != nil {
		return 0, fmt.Errorf("upsert category %q: %w", name, err)
	}
	var id int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM categories WHERE user_id = ? AND name = ?`, userID, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("fetch category %q: %w", name, err)
	}
	return id, nil
}

func isFolder(o opmlOutline) bool {
	return strings.ToLower(o.Type) == "folder" || (o.XMLURL == "" && len(o.Children) > 0)
}

func isFeed(o opmlOutline) bool {
	return o.XMLURL != "" || (o.Text != "" && len(o.Children) == 0)
}

