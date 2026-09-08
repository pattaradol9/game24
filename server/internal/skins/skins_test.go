package skins

import (
	"testing"
)

func TestCatalogSanity(t *testing.T) {
	seen := map[string]bool{}
	for i, s := range Catalog {
		if seen[s.ID] {
			t.Errorf("duplicate skin id %q", s.ID)
		}
		seen[s.ID] = true
		if s.Price < 0 {
			t.Errorf("%s: negative price", s.ID)
		}
		if s.Desc.En == "" || s.Desc.Th == "" {
			t.Errorf("%s: bilingual description required", s.ID)
		}
		switch s.Rarity {
		case "common", "rare", "epic", "legend":
		default:
			t.Errorf("%s: unknown rarity %q", s.ID, s.Rarity)
		}
		// the default skin must be first and free
		if i == 0 && (s.ID != "classic" || s.Price != 0) {
			t.Errorf("first catalog entry must be the free classic skin, got %s", s.ID)
		}
		if i > 0 && s.Price <= 0 {
			t.Errorf("%s: non-default skins must cost coins", s.ID)
		}
	}
	if len(Catalog) != 12 {
		t.Fatalf("catalog has %d skins, want 12", len(Catalog))
	}
	if _, ok := ByID("classic"); !ok {
		t.Error("classic skin must exist")
	}
	if _, ok := ByID("nope"); ok {
		t.Error("unknown skin must not resolve")
	}
}
