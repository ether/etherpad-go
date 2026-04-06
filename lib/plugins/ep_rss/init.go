package ep_rss

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

const feedUrl = "/p/%s/feed"

type FeedCache struct {
	LastEdited int64
	Feed       string
}

type EPRssPlugin struct {
	enabled bool
}

func (E *EPRssPlugin) Name() string {
	return "ep_rss"
}

func (E *EPRssPlugin) Description() string {
	return "Adds RSS feed support to Etherpad"
}

func (E *EPRssPlugin) Init(store *interfaces.EpPluginStore) {
	registerFeedRoutes(store.App, store.PadManager, store.Logger)
}

func (E *EPRssPlugin) SetEnabled(enabled bool) {
	E.enabled = enabled
}

func (E *EPRssPlugin) IsEnabled() bool {
	return E.enabled
}

var (
	// Global feed cache with mutex for thread safety
	feeds   = make(map[string]*FeedCache)
	feedsMu sync.RWMutex
)

const staleTime = 5 * time.Minute

func registerFeedRoutes(app *fiber.App, padManager *pad.Manager, zap *zap.SugaredLogger) {
	// Redirects
	app.Get("/p/:padID/rss", func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusMovedPermanently).To(fmt.Sprintf(feedUrl, c.Params("padID")))
	})
	app.Get("/p/:padID/feed.rss", func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusMovedPermanently).To(fmt.Sprintf(feedUrl, c.Params("padID")))
	})
	app.Get("/p/:padID/atom.xml", func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusMovedPermanently).To(fmt.Sprintf(feedUrl, c.Params("padID")))
	})

	// Main feed handler
	app.Get("/p/:padID/feed", func(c fiber.Ctx) error {
		return handleFeed(c, padManager, zap)
	})
}

func safeTags(str string) string {
	str = strings.ReplaceAll(str, "&", "&amp;")
	str = strings.ReplaceAll(str, "<", "&lt;")
	str = strings.ReplaceAll(str, ">", "&gt;")
	return str
}

func handleFeed(c fiber.Ctx, padManager *pad.Manager, zap *zap.SugaredLogger) error {
	padID := c.Params("padID")
	fullURL := c.BaseURL() + c.OriginalURL()
	padURL := fmt.Sprintf("%s://%s/p/%s", c.Protocol(), c.Hostname(), padID)
	dateString := time.Now().UTC().Format(time.RFC1123)

	var isPublished bool
	var text string

	retrievedPad, err := padManager.GetPad(padID, nil, nil)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	currTS := time.Now()

	feedsMu.Lock()
	if _, ok := feeds[padID]; !ok {
		feeds[padID] = &FeedCache{}
	}
	feedsMu.Unlock()

	if retrievedPad.UpdatedAt != nil && currTS.Sub(*retrievedPad.UpdatedAt) < staleTime {
		isPublished = isAlreadyPublished(padID, *retrievedPad.UpdatedAt)
		if !isPublished {
			feedsMu.Lock()
			feeds[padID].LastEdited = (*retrievedPad.UpdatedAt).UnixMilli()
			feedsMu.Unlock()
		}
	} else if retrievedPad.UpdatedAt != nil {
		feedsMu.RLock()
		hasFeed := feeds[padID].Feed != ""
		feedsMu.RUnlock()

		if !hasFeed {
			isPublished = false
			feedsMu.Lock()
			feeds[padID].LastEdited = (*retrievedPad.UpdatedAt).UnixMilli()
			feedsMu.Unlock()
		} else {
			isPublished = true
		}
	}

	if !isPublished {
		text = safeTags(retrievedPad.Text())
		text = strings.ReplaceAll(text, "\n", "<br/>")
	}

	if isPublished {
		zap.Debug("Sending RSS from memory", "padID", padID)
		feedsMu.RLock()
		feedContent := feeds[padID].Feed
		feedsMu.RUnlock()
		c.Set("Content-Type", "application/rss+xml; charset=utf-8")
		return c.SendString(feedContent)
	}

	zap.Debug("Building RSS", "padID", padID)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<rss version="2.0"` + "\n")
	b.WriteString(`   xmlns:content="http://purl.org/rss/1.0/modules/content/"` + "\n")
	b.WriteString(`   xmlns:atom="http://www.w3.org/2005/Atom"` + "\n")
	b.WriteString(`>` + "\n")
	b.WriteString(`<channel>` + "\n")
	b.WriteString(fmt.Sprintf(`<title>%s</title>`+"\n", html.EscapeString(padID)))
	b.WriteString(fmt.Sprintf(`<atom:link href="%s" rel="self" type="application/rss+xml" />`+"\n", html.EscapeString(fullURL)))
	b.WriteString(fmt.Sprintf(`<link>%s</link>`+"\n", html.EscapeString(padURL)))
	b.WriteString(`<description/>` + "\n")
	b.WriteString(`<language>en-us</language>` + "\n")
	b.WriteString(fmt.Sprintf(`<pubDate>%s</pubDate>`+"\n", dateString))
	b.WriteString(fmt.Sprintf(`<lastBuildDate>%s</lastBuildDate>`+"\n", dateString))
	b.WriteString(`<item>` + "\n")
	b.WriteString(`<title>` + "\n")
	b.WriteString(html.EscapeString(padID) + "\n")
	b.WriteString(`</title>` + "\n")
	b.WriteString(`<description>` + "\n")
	b.WriteString(fmt.Sprintf(`<![CDATA[%s]]>`+"\n", text))
	b.WriteString(`</description>` + "\n")
	b.WriteString(fmt.Sprintf(`<link>%s</link>`+"\n", html.EscapeString(padURL)))
	b.WriteString(`</item>` + "\n")
	b.WriteString(`</channel>` + "\n")
	b.WriteString(`</rss>`)

	feedContent := b.String()
	feedsMu.Lock()
	feeds[padID].Feed = feedContent
	feedsMu.Unlock()

	c.Set("Content-Type", "application/rss+xml; charset=utf-8")
	return c.SendString(feedContent)
}

func isAlreadyPublished(padID string, editTime time.Time) bool {
	feedsMu.RLock()
	defer feedsMu.RUnlock()
	feed, ok := feeds[padID]
	if !ok {
		return false
	}
	return feed.LastEdited == editTime.UnixMilli()
}

var _ interfaces.EpPlugin = (*EPRssPlugin)(nil)
