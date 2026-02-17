package discovery

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const fetchTimeout = 15 * time.Second

func FetchPage(ctx context.Context, url string) (FetchedPage, error) {
	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return FetchedPage{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LanguageApp/1.0)")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return FetchedPage{}, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FetchedPage{}, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return FetchedPage{}, fmt.Errorf("parse html from %s: %w", url, err)
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())

	// Remove scripts and styles before extracting text
	doc.Find("script, style, nav, header, footer").Remove()

	var body string
	// Try article-specific selectors first, fall back to body
	for _, sel := range []string{"article", "main", ".article-content", ".post-content", "body"} {
		node := doc.Find(sel).First()
		if node.Length() > 0 {
			body = strings.TrimSpace(node.Text())
			if body != "" {
				break
			}
		}
	}

	// Normalize whitespace
	body = collapseWhitespace(body)

	return FetchedPage{URL: url, Title: title, Body: body}, nil
}

func HasCJKContent(text string) bool {
	cjkCount := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			cjkCount++
			if cjkCount >= 20 {
				return true
			}
		}
	}
	return false
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}
