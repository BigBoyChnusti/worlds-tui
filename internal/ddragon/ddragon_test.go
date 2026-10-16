package ddragon

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchVersion(t *testing.T) {
	versions := []string{"16.20.1", "16.19.1", "15.20.1", "15.2.1"}
	cases := map[string]string{
		"15.20.719.545": "15.20.1",
		"15.2.600.1":    "15.2.1",
		"16.19.5":       "16.19.1",
		"99.1.0":        "16.20.1",
		"":              "16.20.1",
	}
	for patch, want := range cases {
		if got := MatchVersion(patch, versions); got != want {
			t.Errorf("MatchVersion(%q) = %q, want %q", patch, got, want)
		}
	}
}

func TestNamesFallBackToIDs(t *testing.T) {
	var d *Data
	if got := d.ItemName(3031); got != "3031" {
		t.Errorf("ItemName on nil data = %q", got)
	}
	d = &Data{Items: map[int]string{3031: "Infinity Edge"}, Champions: map[string]string{"JarvanIV": "Jarvan IV"}}
	if d.ItemName(3031) != "Infinity Edge" || d.ChampionName("JarvanIV") != "Jarvan IV" || d.ChampionName("New") != "New" {
		t.Error("lookup mismatch")
	}
}

func TestChampionIconFetchAndCache(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(1, 0, color.RGBA{200, 100, 50, 255})
	var body bytes.Buffer
	if err := png.Encode(&body, src); err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/cdn/15.20.1/img/champion/MonkeyKing.png" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(body.Bytes())
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := NewClient()
	c.BaseURL, c.CacheDir = srv.URL, dir

	for i := range 2 {
		img, err := c.ChampionIcon(context.Background(), "15.20.1", "MonkeyKing")
		if err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
		if r, g, b, _ := img.At(1, 0).RGBA(); r>>8 != 200 || g>>8 != 100 || b>>8 != 50 {
			t.Errorf("fetch %d: pixel = %d,%d,%d", i, r>>8, g>>8, b>>8)
		}
	}
	if hits != 1 {
		t.Errorf("server hit %d times, want 1 (second read from cache)", hits)
	}
	if _, err := os.Stat(filepath.Join(dir, "15.20.1-icon-MonkeyKing.png")); err != nil {
		t.Errorf("icon not cached: %v", err)
	}

	if _, err := c.ChampionIcon(context.Background(), "15.20.1", "Nobody"); err == nil {
		t.Error("missing icon returned no error")
	}
	if _, err := os.Stat(filepath.Join(dir, "15.20.1-icon-Nobody.png")); err == nil {
		t.Error("failed download was cached")
	}
	if _, err := c.ChampionIcon(context.Background(), "15.20.1", "../x"); err == nil {
		t.Error("path-like champion key accepted")
	}
}
