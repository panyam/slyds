package core

import (
	"io/fs"
	"os"
	"path/filepath"
)

// OSFS implements DeckFS backed by the local filesystem.
// The root is an absolute directory path; all operations are relative to it.
type OSFS struct {
	Root string
}

// NewOSFS creates a DeckFS backed by the local filesystem at the given root.
func NewOSFS(root string) *OSFS {
	return &OSFS{Root: root}
}

// Open implements fs.FS.
func (f *OSFS) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(f.Root, name))
}

// ReadDir implements fs.ReadDirFS.
func (f *OSFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(filepath.Join(f.Root, name))
}

// ReadFile implements fs.ReadFileFS.
func (f *OSFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(f.Root, name))
}

// Stat implements fs.StatFS.
func (f *OSFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(filepath.Join(f.Root, name))
}

// WriteFile implements DeckFS.
func (f *OSFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filepath.Join(f.Root, name), data, perm)
}

// MkdirAll implements DeckFS.
func (f *OSFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(filepath.Join(f.Root, path), perm)
}

// Remove implements DeckFS.
func (f *OSFS) Remove(name string) error {
	return os.Remove(filepath.Join(f.Root, name))
}

// Rename implements DeckFS.
func (f *OSFS) Rename(oldname, newname string) error {
	return os.Rename(filepath.Join(f.Root, oldname), filepath.Join(f.Root, newname))
}

// AbsPath returns the absolute path for a relative name within the FS.
// This is OS-specific and only available on OSFS (not on S3/IndexedDB backends).
func (f *OSFS) AbsPath(name string) string {
	return filepath.Join(f.Root, name)
}
