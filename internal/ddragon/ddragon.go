// Package ddragon resolves item and champion ids to names using Riot's
// Data Dragon static data, cached on disk per game version.
package ddragon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://ddragon.leagueoflegends.com"

// Data holds the lookups for one Data Dragon version.
type Data struct {
	Version   string
	Items     map[int]string
	Champions map[string]string
}

// ItemName returns the item's name, or its id when unknown.
func (d *Data) ItemName(id int) string {
	if d != nil {
		if n, ok := d.Items[id]; ok {
			return n
		}
	}
	return strconv.Itoa(id)
}

// ChampionName returns the display name for a champion key such as
// "JarvanIV", or the key itself when unknown.
func (d *Data) ChampionName(key string) string {
	if d != nil {
		if n, ok := d.Champions[key]; ok {
			return n
		}
	}
	return key
}

type Client struct {
	HTTP     *http.Client
	BaseURL  string
	CacheDir string

	mu    sync.Mutex
	byVer map[string]*Data
}

// NewClient caches downloads under the user cache directory; caching is
// skipped when that directory is unavailable.
func NewClient() *Client {
	dir, err := os.UserCacheDir()
	if err == nil {
		dir = filepath.Join(dir, "worlds-tui", "ddragon")
	} else {
		dir = ""
	}
	return &Client{
		HTTP:     &http.Client{Timeout: 20 * time.Second},
		BaseURL:  DefaultBaseURL,
		CacheDir: dir,
		byVer:    map[string]*Data{},
	}
}

// MatchVersion maps a game patch such as "15.20.719.545" to the Data Dragon
// version with the same major.minor, falling back to the newest version.
func MatchVersion(patch string, versions []string) string {
	parts := strings.Split(patch, ".")
	if len(parts) >= 2 {
		prefix := parts[0] + "." + parts[1] + "."
		for _, v := range versions {
			if strings.HasPrefix(v, prefix) {
				return v
			}
		}
	}
	if len(versions) > 0 {
		return versions[0]
	}
	return ""
}

// ForPatch returns the data for the game patch, downloading it on first
// use.
func (c *Client) ForPatch(ctx context.Context, patch string) (*Data, error) {
	var versions []string
	if err := c.fetchJSON(ctx, "/api/versions.json", "versions.json", time.Hour*12, &versions); err != nil {
		return nil, err
	}
	ver := MatchVersion(patch, versions)
	if ver == "" {
		return nil, fmt.Errorf("ddragon: no versions available")
	}

	c.mu.Lock()
	d, ok := c.byVer[ver]
	c.mu.Unlock()
	if ok {
		return d, nil
	}

	var items struct {
		Data map[string]struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.fetchJSON(ctx, "/cdn/"+ver+"/data/en_US/item.json", ver+"-item.json", 0, &items); err != nil {
		return nil, err
	}
	var champs struct {
		Data map[string]struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.fetchJSON(ctx, "/cdn/"+ver+"/data/en_US/champion.json", ver+"-champion.json", 0, &champs); err != nil {
		return nil, err
	}

	d = &Data{Version: ver, Items: map[int]string{}, Champions: map[string]string{}}
	for k, v := range items.Data {
		if id, err := strconv.Atoi(k); err == nil {
			d.Items[id] = v.Name
		}
	}
	for k, v := range champs.Data {
		d.Champions[k] = v.Name
	}
	c.mu.Lock()
	c.byVer[ver] = d
	c.mu.Unlock()
	return d, nil
}

// ChampionIcon returns the square icon of a champion key such as
// "MonkeyKing" for a Data Dragon version. Icons never change within a
// version, so the cached file never expires.
func (c *Client) ChampionIcon(ctx context.Context, version, champion string) (image.Image, error) {
	if version == "" || champion == "" || strings.ContainsAny(champion, "/\\.") {
		return nil, fmt.Errorf("ddragon: bad icon %q for version %q", champion, version)
	}
	var img image.Image
	path := "/cdn/" + version + "/img/champion/" + champion + ".png"
	err := c.fetch(ctx, path, version+"-icon-"+champion+".png", 0, func(b []byte) error {
		var err error
		img, err = png.Decode(bytes.NewReader(b))
		return err
	})
	if err != nil {
		return nil, err
	}
	return img, nil
}

// fetchJSON decodes path into out, reading from the cache file when it is
// younger than maxAge (0 means it never expires).
func (c *Client) fetchJSON(ctx context.Context, path, cacheName string, maxAge time.Duration, out any) error {
	return c.fetch(ctx, path, cacheName, maxAge, func(b []byte) error { return json.Unmarshal(b, out) })
}

// fetch downloads path and hands the body to decode, reading from the cache file when it is
// younger than maxAge (0 means it never expires). decode must accept the
// body; a cache file it rejects is downloaded again, and a download it
// rejects is not cached.
func (c *Client) fetch(ctx context.Context, path, cacheName string, maxAge time.Duration, decode func([]byte) error) error {
	cachePath := ""
	if c.CacheDir != "" {
		cachePath = filepath.Join(c.CacheDir, cacheName)
		if st, err := os.Stat(cachePath); err == nil && (maxAge == 0 || time.Since(st.ModTime()) < maxAge) {
			if b, err := os.ReadFile(cachePath); err == nil && decode(b) == nil {
				return nil
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ddragon: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ddragon %s: HTTP %d", path, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ddragon: %w", err)
	}
	if err := decode(b); err != nil {
		return fmt.Errorf("ddragon %s: decode: %w", path, err)
	}
	if cachePath != "" && os.MkdirAll(c.CacheDir, 0o755) == nil {
		_ = os.WriteFile(cachePath, b, 0o644)
	}
	return nil
}
