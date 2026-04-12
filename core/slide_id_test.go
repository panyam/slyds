package core

import (
	"strings"
	"testing"
)

// TestNewSlideID_Format verifies that newSlideID returns the expected
// "sl_" + 8 hex chars format. This is the contract agents and docs
// rely on when explaining "what does a slide_id look like".
func TestNewSlideID_Format(t *testing.T) {
	id := newSlideID()

	if !strings.HasPrefix(id, SlideIDPrefix) {
		t.Errorf("id %q missing %q prefix", id, SlideIDPrefix)
	}

	// sl_ (3) + 8 hex chars = 11 total
	if len(id) != len(SlideIDPrefix)+8 {
		t.Errorf("id %q has length %d, want %d", id, len(id), len(SlideIDPrefix)+8)
	}

	hexPart := strings.TrimPrefix(id, SlideIDPrefix)
	for _, r := range hexPart {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			t.Errorf("id %q contains non-hex char %q", id, r)
		}
	}
}

// TestNewSlideID_Unique verifies that generating many slide_ids in
// sequence produces all unique values. At 32 random bits, 100 calls
// should effectively never collide; this test catches a broken RNG
// or a seeding bug.
func TestNewSlideID_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := newSlideID()
		if seen[id] {
			t.Errorf("newSlideID collision at iteration %d: %q", i, id)
		}
		seen[id] = true
	}
}

// TestUniqueSlideID_SkipsUsed verifies that uniqueSlideID never returns
// an id already present in the used map. Seeds the used map with a
// known-good id and calls uniqueSlideID once; asserts the result is
// different AND is recorded in the map for the next call.
func TestUniqueSlideID_SkipsUsed(t *testing.T) {
	used := map[string]bool{"sl_abcd1234": true}
	id := uniqueSlideID(used)
	if id == "sl_abcd1234" {
		t.Errorf("uniqueSlideID returned the used id")
	}
	if !used[id] {
		t.Errorf("uniqueSlideID did not record the returned id in the used map")
	}
}

// TestIsSlideID verifies the prefix check used by ResolveSlide to
// fast-path slide_id lookups.
func TestIsSlideID(t *testing.T) {
	cases := []struct {
		ref  string
		want bool
	}{
		{"sl_a1b2c3d4", true},
		{"sl_", true}, // prefix-only still counts; lookup will fail
		{"sl_anything", true},
		{"intro", false},
		{"03-intro.html", false},
		{"3", false},
		{"", false},
		{"SL_a1b2c3d4", false}, // case-sensitive: the prefix is literal sl_
	}
	for _, c := range cases {
		t.Run(c.ref, func(t *testing.T) {
			if got := IsSlideID(c.ref); got != c.want {
				t.Errorf("IsSlideID(%q) = %v, want %v", c.ref, got, c.want)
			}
		})
	}
}
