package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// SlideIDPrefix is the literal prefix slyds assigns to every generated
// slide_id. It serves two purposes:
//
//  1. Disambiguation: ResolveSlide can fast-path any reference starting
//     with this prefix to the slide_id lookup, skipping slug/position
//     matching.
//  2. Human readability: agents and users seeing `sl_a1b2c3d4` in logs
//     or tool responses know it's a slide identifier without ambiguity.
//
// The prefix is reserved — callers of InsertSlide that pass a name
// starting with "sl_" are forbidden (would cause lookup ambiguity).
const SlideIDPrefix = "sl_"

// slideIDRandomBytes is the number of random bytes used to generate a
// slide_id. 4 bytes → 8 hex chars → 32 bits. At 100 slides per deck,
// birthday collision probability is ~1e-6; we retry on collision, so
// this is effectively zero in practice.
const slideIDRandomBytes = 4

// newSlideID returns a freshly-generated slide identifier. Format is
// "sl_" followed by hex-encoded random bytes (see SlideIDPrefix and
// slideIDRandomBytes). Callers that need uniqueness within a deck
// should use uniqueSlideID instead.
func newSlideID() string {
	var b [slideIDRandomBytes]byte
	// crypto/rand.Read is documented to never return a short read and
	// to only error if the system entropy pool is unavailable — which
	// on any sane OS is a hard failure the rest of the program can't
	// recover from anyway. Treat an error here as a panic condition.
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("slyds: failed to read crypto/rand for slide_id: %v", err))
	}
	return SlideIDPrefix + hex.EncodeToString(b[:])
}

// uniqueSlideID returns a newly-generated slide_id that doesn't collide
// with any id in the provided set. Retries on collision; at the scale
// slyds operates (dozens to low hundreds of slides per deck), a first-
// attempt collision is vanishingly rare, so the loop terminates almost
// immediately. The returned id is added to the used map so subsequent
// calls with the same map stay consistent.
func uniqueSlideID(used map[string]bool) string {
	for {
		id := newSlideID()
		if !used[id] {
			used[id] = true
			return id
		}
	}
}

// IsSlideID reports whether s looks like a slyds-generated slide_id.
// Used by ResolveSlide to fast-path id lookups. It only checks the
// prefix and does not validate the hex portion — a value like
// "sl_anything" is still treated as a slide_id candidate so that a
// typo produces a "slide id not found" error rather than falling
// through to slug/position matching and returning a confusing result.
func IsSlideID(s string) bool {
	return strings.HasPrefix(s, SlideIDPrefix)
}
