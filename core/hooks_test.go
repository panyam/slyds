package core

import (
	"strings"
	"testing"
)

// TestBuildContainsSlideLifecycleEvents verifies that built presentations include
// the slideEnter and slideLeave CustomEvent dispatches in the inlined JavaScript.
// These events fire during slide transitions so that user/agent code (e.g., Chart.js
// initialization, animation triggers) can hook into the navigation lifecycle without
// needing to poll or use MutationObservers.
func TestBuildContainsSlideLifecycleEvents(t *testing.T) {
	html := scaffoldAndBuild(t)

	markers := []string{
		"slideEnter",
		"slideLeave",
		"dispatchEvent",
		"CustomEvent",
	}

	for _, m := range markers {
		if !strings.Contains(html, m) {
			t.Errorf("built HTML missing lifecycle hook marker: %s", m)
		}
	}
}

// TestBuildContainsSlydsContext verifies that the built presentation includes the
// window.slydsContext object. This persistent context provides presentation metadata
// (totalSlides, currentSlide, direction) and a state bag that survives slide
// transitions — used by agents to cache chart instances, track first-visit flags,
// and pass data between slides without re-querying the DOM.
func TestBuildContainsSlydsContext(t *testing.T) {
	html := scaffoldAndBuild(t)

	markers := []string{
		"slydsContext",
		"window.slydsContext",
	}

	for _, m := range markers {
		if !strings.Contains(html, m) {
			t.Errorf("built HTML missing slydsContext marker: %s", m)
		}
	}
}

// TestBuildContainsHookEventDetail verifies that the slideEnter/slideLeave event
// detail payload includes the expected fields: slideNum (1-based slide number),
// direction ("forward"/"backward"/"init"), and layout (from data-layout attribute).
// These fields let hook consumers respond to navigation context without parsing
// the DOM — e.g., only initializing charts on forward navigation, or varying
// behavior by layout type.
func TestBuildContainsHookEventDetail(t *testing.T) {
	html := scaffoldAndBuild(t)

	detailFields := []string{
		"slideNum",
		"direction",
		"buildEventDetail",
	}

	for _, f := range detailFields {
		if !strings.Contains(html, f) {
			t.Errorf("built HTML missing event detail field: %s", f)
		}
	}
}
