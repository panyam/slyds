package core

import (
	"strings"
	"testing"

	"github.com/panyam/templar"
)

func TestWriteAndReadManifest(t *testing.T) {
	mfs := templar.NewMemFS()
	m := Manifest{Theme: "dark", Title: "My Talk"}

	if err := WriteManifestFS(mfs, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifestFS(mfs)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got.Theme != m.Theme || got.Title != m.Title {
		t.Errorf("got %+v, want %+v", got, m)
	}
}

func TestReadManifestNotFound(t *testing.T) {
	mfs := templar.NewMemFS()
	_, err := ReadManifestFS(mfs)
	if err != ErrManifestNotFound {
		t.Errorf("got error %v, want ErrManifestNotFound", err)
	}
}

func TestManifestFileContents(t *testing.T) {
	mfs := templar.NewMemFS()
	m := Manifest{Theme: "corporate", Title: "Q4 Review"}

	if err := WriteManifestFS(mfs, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	data, err := mfs.ReadFile(".slyds.yaml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "theme: corporate") || !strings.Contains(content, "title: Q4 Review") {
		t.Errorf("unexpected manifest content:\n%s", content)
	}
}
