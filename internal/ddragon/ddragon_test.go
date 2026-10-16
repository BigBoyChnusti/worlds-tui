package ddragon

import "testing"

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
