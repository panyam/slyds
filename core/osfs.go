package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/panyam/templar"
)

// FindDeckRoot resolves an absolute path to a deck root directory on the local filesystem.
// Returns an error if dir does not contain index.html.
// This is the entry point for CLI usage — after this, use OpenDeck(templar.NewLocalFS(root)).
func FindDeckRoot(dir string) (string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); os.IsNotExist(err) {
		return "", fmt.Errorf("no index.html found in %s — is this a slyds presentation? Run 'slyds init' to create one", root)
	}
	return root, nil
}

// OpenDeckDir is a convenience for opening a deck from a local directory path.
func OpenDeckDir(dir string) (*Deck, error) {
	root, err := FindDeckRoot(dir)
	if err != nil {
		return nil, err
	}
	return OpenDeck(templar.NewLocalFS(root))
}

// OpenDeckCwd is a convenience for opening a deck from the current working directory.
func OpenDeckCwd() (*Deck, error) {
	return OpenDeckDir(".")
}
