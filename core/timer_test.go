package core

import (
	"strings"
	"testing"
)

// scaffoldAndBuild creates a test deck on MemFS, builds it, and returns the HTML.
func scaffoldAndBuild(t *testing.T) string {
	t.Helper()
	d, _ := scaffoldMem(t, "Timer Test", withSlides(4))
	result, err := d.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	return result.HTML
}

func TestBuildContainsTimerFeatures(t *testing.T) {
	html := scaffoldAndBuild(t)

	timerFunctions := []string{
		"toggleTimer",
		"startTimer",
		"pauseTimer",
		"formatTime",
		"getElapsedMs",
	}

	for _, fn := range timerFunctions {
		if !strings.Contains(html, fn) {
			t.Errorf("built HTML missing timer function: %s", fn)
		}
	}
}

func TestBuildContainsNotesTimerUI(t *testing.T) {
	html := scaffoldAndBuild(t)

	uiElements := []string{
		"notesTimer",
		"notesReadingTime",
		"notesRemaining",
		"notesTimerToggle",
		"timer-bar",
	}

	for _, el := range uiElements {
		if !strings.Contains(html, el) {
			t.Errorf("built HTML missing notes timer UI element: %s", el)
		}
	}
}

func TestBuildTimerKeyboardShortcut(t *testing.T) {
	html := scaffoldAndBuild(t)

	if !strings.Contains(html, "e.key === 't'") && !strings.Contains(html, `e.key === 'T'`) {
		t.Error("built HTML missing T key handler for timer toggle")
	}
}

func TestBuildReadingTimeComputation(t *testing.T) {
	html := scaffoldAndBuild(t)

	readingTimeFunctions := []string{
		"computeReadingTimes",
		"slideReadingTimes",
		"getReadingTime",
		"getRemainingReadingTime",
		"formatReadingTime",
	}

	for _, fn := range readingTimeFunctions {
		if !strings.Contains(html, fn) {
			t.Errorf("built HTML missing reading time function: %s", fn)
		}
	}
}
