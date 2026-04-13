package core

import (
	"crypto/sha256"
	"fmt"
)

// ContentVersion computes a version string from content bytes.
// Returns the first 16 hex characters of the SHA-256 hash.
// Used for optimistic concurrency — agents pass this version back
// on mutations and the server rejects stale writes.
func ContentVersion(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8]) // 8 bytes = 16 hex chars
}

// SlideVersion returns the content version of the slide at the given
// 1-based position. Reads the slide file through the FS abstraction
// and hashes its content.
func (d *Deck) SlideVersion(position int) (string, error) {
	files, err := d.SlideFilenames()
	if err != nil {
		return "", err
	}
	if position < 1 || position > len(files) {
		return "", fmt.Errorf("slide %d out of range (deck has %d slides)", position, len(files))
	}
	data, err := d.FS.ReadFile("slides/" + files[position-1])
	if err != nil {
		return "", fmt.Errorf("read slide %d: %w", position, err)
	}
	return ContentVersion(data), nil
}

// DeckVersion returns the content version of the deck's index.html.
// Changes when slides are added, removed, or reordered.
func (d *Deck) DeckVersion() (string, error) {
	data, err := d.FS.ReadFile("index.html")
	if err != nil {
		return "", fmt.Errorf("read index.html: %w", err)
	}
	return ContentVersion(data), nil
}
