package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

)

func TestBuildEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Scaffold a presentation
	slug, err := Create("Build Test", 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	root := filepath.Join(tmp, slug)

	// Build it
	result, err := Build(root)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// The built HTML should:
	// 1. Have no {{# include #}} directives (all resolved)
	if strings.Contains(result.HTML, "{{#") {
		t.Error("built HTML still contains templar directives")
	}

	// 2. Contain all slide content (title + content + closing = 3 slides)
	slideCount := strings.Count(result.HTML, `class="slide`)
	// Expect at least 3 (the actual div.slide elements, not CSS references)
	if slideCount < 3 {
		t.Errorf("expected at least 3 slide references, got %d", slideCount)
	}

	// 3. Have CSS inlined (no <link> tags)
	if strings.Contains(result.HTML, `<link rel="stylesheet"`) {
		t.Error("built HTML still contains <link> stylesheet tags")
	}
	if !strings.Contains(result.HTML, "<style>") {
		t.Error("built HTML missing inlined <style> tags")
	}

	// 4. Have JS inlined (no <script src> tags)
	if strings.Contains(result.HTML, `<script src=`) {
		t.Error("built HTML still contains <script src> tags")
	}
	if !strings.Contains(result.HTML, "showSlide") {
		t.Error("built HTML missing inlined JS (showSlide function)")
	}

	// 5. Have the presentation title
	if !strings.Contains(result.HTML, "Build Test") {
		t.Error("built HTML missing presentation title")
	}
}
