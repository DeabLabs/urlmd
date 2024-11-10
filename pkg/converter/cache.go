package converter

import (
	"encoding/json"
	"time"
)

// CacheEntry represents a cached conversion result
type CacheEntry struct {
	URL         string    `json:"url"`
	Markdown    string    `json:"markdown"`
	LastFetched time.Time `json:"last_fetched"`
}

func (c *Converter) getFromCache(url string) (*CacheEntry, error) {
	var data string
	var entry CacheEntry

	err := c.db.QueryRow("SELECT data FROM cache WHERE url = ?", url).Scan(&data)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (c *Converter) saveToCache(entry *CacheEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(
		"INSERT OR REPLACE INTO cache (url, data, last_fetched) VALUES (?, ?, ?)",
		entry.URL, string(data), entry.LastFetched,
	)
	return err
}
