package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadManifest(t *testing.T) {
	dir := t.TempDir()
	m := Manifest{Theme: "dark", Title: "My Talk"}

	if err := WriteManifest(dir, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got.Theme != m.Theme || got.Title != m.Title {
		t.Errorf("got %+v, want %+v", got, m)
	}
}

func TestReadManifestNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadManifest(dir)
	if err != ErrManifestNotFound {
		t.Errorf("got error %v, want ErrManifestNotFound", err)
	}
}

func TestManifestPath(t *testing.T) {
	got := ManifestPath("/foo/bar")
	want := filepath.Join("/foo/bar", ".slyds.yaml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestManifestFileContents(t *testing.T) {
	dir := t.TempDir()
	m := Manifest{Theme: "corporate", Title: "Q4 Review"}

	if err := WriteManifest(dir, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	data, err := os.ReadFile(ManifestPath(dir))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !contains(content, "theme: corporate") || !contains(content, "title: Q4 Review") {
		t.Errorf("unexpected manifest content:\n%s", content)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
