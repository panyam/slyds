package builder

import (
	"strings"
	"testing"
)

// TestBuildContainsFloatCSS verifies that built presentations include the
// .slide-floater CSS class with position:absolute positioning. This class
// provides the primitive for pinned overlays (footers, watermarks, logos)
// that stay fixed within the slide regardless of content scrolling.
func TestBuildContainsFloatCSS(t *testing.T) {
	html := scaffoldAndBuild(t)

	markers := []string{
		"slide-floater",
		"position: absolute",
	}

	for _, m := range markers {
		if !strings.Contains(html, m) {
			t.Errorf("built HTML missing float CSS marker: %s", m)
		}
	}
}

// TestBuildContainsFloatSlot verifies that scaffolded presentations include
// data-slot="floater" in layout templates that support floating overlays
// (content, two-col, closing). This slot is the target for slyds query
// when agents inject footers, watermarks, or logos into slides.
func TestBuildContainsFloatSlot(t *testing.T) {
	html := scaffoldAndBuild(t)

	// scaffoldAndBuild creates a 4-slide deck: title + 2 content + closing
	// content and closing layouts should have the float slot
	if !strings.Contains(html, `data-slot="floater"`) {
		t.Error("built HTML missing data-slot=\"float\" in scaffolded slides")
	}
}

// TestBuildFloatNotInTitleLayout verifies that title and section layout
// slides do NOT include a float slot by default. These focused layouts
// (full-screen title, section divider) should remain minimal — authors
// can always add a float element manually if needed.
func TestBuildFloatNotInTitleLayout(t *testing.T) {
	html := scaffoldAndBuild(t)

	// Find the title slide (first slide) and check it has no float slot.
	// The title slide uses data-layout="title" — extract that section.
	titleIdx := strings.Index(html, `data-layout="title"`)
	if titleIdx == -1 {
		t.Fatal("built HTML missing title layout slide")
	}

	// Find the next slide after the title slide
	nextSlideIdx := strings.Index(html[titleIdx+1:], `class="slide"`)
	if nextSlideIdx == -1 {
		// Only one slide with title layout — check from title to end
		nextSlideIdx = len(html) - titleIdx - 1
	}

	titleSection := html[titleIdx : titleIdx+1+nextSlideIdx]
	if strings.Contains(titleSection, `data-slot="floater"`) {
		t.Error("title layout slide should not contain a float slot by default")
	}
}
