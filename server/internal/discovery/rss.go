package discovery

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// rssFeed describes a western news outlet that publishes content in Chinese.
type rssFeed struct {
	URL  string
	Name string
}

// chineseRSSFeeds is the curated list of RSS sources.
// All feeds publish simplified Chinese (普通话) and are freely accessible worldwide.
var chineseRSSFeeds = []rssFeed{
	{
		URL:  "https://feeds.bbci.co.uk/zhongwen/simp/rss.xml",
		Name: "BBC Chinese",
	},
	{
		URL:  "https://www.voachinese.com/api/zmobj-rss-gen?zone=1547&count=20",
		Name: "VOA Chinese",
	},
	{
		URL:  "https://rss.dw.com/xml/rss-zh-all",
		Name: "DW Chinese",
	},
}

const rssMaxTotal = 20

// fetchRSSPages fetches articles from the curated Chinese RSS feeds and returns
// FetchedPage values whose Body contains the title and description text.
// This is enough for HasCJKContent and ScoreArticle without fetching the full page HTML.
func fetchRSSPages(ctx context.Context, existingURLs []string) ([]FetchedPage, error) {
	existing := make(map[string]bool, len(existingURLs))
	for _, u := range existingURLs {
		existing[u] = true
	}

	fp := gofeed.NewParser()
	fp.UserAgent = "Mozilla/5.0 (compatible; language-app-discovery/1.0)"

	var pages []FetchedPage

	for _, feed := range chineseRSSFeeds {
		if len(pages) >= rssMaxTotal {
			break
		}

		feedCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		parsed, err := fp.ParseURLWithContext(feed.URL, feedCtx)
		cancel()

		if err != nil {
			log.Printf("rss fetch failed: feed=%s err=%v", feed.Name, err)
			continue
		}

		for _, item := range parsed.Items {
			if len(pages) >= rssMaxTotal {
				break
			}
			if item.Link == "" || existing[item.Link] {
				continue
			}

			// Use title + description as body so HasCJKContent and ScoreArticle
			// have real Chinese text without needing to fetch the full article HTML.
			body := strings.TrimSpace(item.Title + " " + item.Description)

			pages = append(pages, FetchedPage{
				URL:   item.Link,
				Title: item.Title,
				Body:  body,
			})
			existing[item.Link] = true // deduplicate within this run
		}

		log.Printf("rss fetched: feed=%s items=%d", feed.Name, len(parsed.Items))
	}

	return pages, nil
}
