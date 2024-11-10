package converter

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	htmlToMarkdownConverter "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	_ "github.com/mattn/go-sqlite3"
	"github.com/playwright-community/playwright-go"
)

// Config holds the configuration for the converter
type Config struct {
	CacheDuration time.Duration
	CachePath     string
	Timeout       time.Duration
	UserAgent     string
}

// Converter handles webpage to markdown conversion with caching
type Converter struct {
	db     *sql.DB
	pw     *playwright.Playwright
	config Config
	conv   *htmlToMarkdownConverter.Converter
	lp     *linkPlugin
}

// NewConverter creates a new converter instance
func NewConverter(config Config) (*Converter, error) {
	// Initialize cache directory if it doesn't exist
	// The directory is everything up to the filename
	dir := filepath.Dir(config.CachePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	// Initialize SQLite
	db, err := sql.Open("sqlite3", config.CachePath)
	if err != nil {
		return nil, err
	}

	// Create cache table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cache (
			url TEXT PRIMARY KEY,
			data TEXT,
			last_fetched TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}

	// Initialize Playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	// Initialize converter library instance
	lp := NewLinkPlugin("")
	lpa := lp.(*linkPlugin)
	converter := htmlToMarkdownConverter.NewConverter(htmlToMarkdownConverter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		lp,
	))

	return &Converter{
		db:     db,
		pw:     pw,
		config: config,
		conv:   converter,
		lp:     lpa,
	}, nil
}

// Convert fetches a webpage and converts it to markdown, using cache when available
func (c *Converter) Convert(ctx context.Context, url string) (string, error) {
	// Check cache first
	entry, err := c.getFromCache(url)
	if err == nil && time.Since(entry.LastFetched) < c.config.CacheDuration {
		return entry.Markdown, nil
	}

	// Fetch and convert if not in cache or expired
	markdown, err := c.fetchAndConvert(ctx, url)
	if err != nil {
		return "", err
	}

	// Update cache
	entry = &CacheEntry{
		URL:         url,
		Markdown:    markdown,
		LastFetched: time.Now(),
	}
	if err := c.saveToCache(entry); err != nil {
		return "", err
	}

	return markdown, nil
}

func (c *Converter) fetchAndConvert(ctx context.Context, url string) (string, error) {
	// Launch browser
	browser, err := c.pw.Chromium.Launch()
	if err != nil {
		return "", err
	}
	defer browser.Close()

	// Create page
	page, err := browser.NewPage()
	if err != nil {
		return "", err
	}

	// Set timeout and user agent
	if c.config.UserAgent != "" {
		err = page.SetExtraHTTPHeaders(map[string]string{
			"User-Agent": c.config.UserAgent,
		})
		if err != nil {
			return "", err
		}
	}

	// Navigate to page
	if _, err = page.Goto(url); err != nil {
		return "", err
	}

	// Wait for network idle
	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		return "", err
	}

	// Get HTML content
	content, err := page.Content()
	if err != nil {
		return "", err
	}

	// Set baseURL for link plugin
	// This is used to convert relative links to absolute links
	c.lp.baseURL = url

	// Convert HTML to markdown using your preferred library
	// This is a placeholder - you'll need to implement the actual conversion
	markdown, err := c.conv.ConvertString(content)
	if err != nil {
		// Reset baseURL for link plugin
		c.lp.baseURL = ""
		return "", err
	}

	// Reset baseURL for link plugin
	c.lp.baseURL = ""

	return markdown, nil
}

// Close cleans up resources
func (c *Converter) Close() error {
	if err := c.db.Close(); err != nil {
		return err
	}
	return c.pw.Stop()
}
