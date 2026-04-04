package core

import (
	"strings"
	"testing"
)

func TestBuildEndToEnd(t *testing.T) {
	d, _ := scaffoldMem(t, "Build Test")

	result, err := d.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// 1. No unresolved include directives
	if strings.Contains(result.HTML, "{{#") {
		t.Error("built HTML still contains templar directives")
	}

	// 2. Contains all slide content
	slideCount := strings.Count(result.HTML, `class="slide`)
	if slideCount < 3 {
		t.Errorf("expected at least 3 slide references, got %d", slideCount)
	}

	// 3. CSS inlined
	if strings.Contains(result.HTML, `<link rel="stylesheet"`) {
		t.Error("built HTML still contains <link> stylesheet tags")
	}
	if !strings.Contains(result.HTML, "<style>") {
		t.Error("built HTML missing inlined <style> tags")
	}

	// 4. JS inlined
	if strings.Contains(result.HTML, `<script src=`) {
		t.Error("built HTML still contains <script src> tags")
	}
	if !strings.Contains(result.HTML, "showSlide") {
		t.Error("built HTML missing inlined JS (showSlide function)")
	}

	// 5. Has title
	if !strings.Contains(result.HTML, "Build Test") {
		t.Error("built HTML missing presentation title")
	}
}
