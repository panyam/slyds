package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

)

// scaffoldAndBuild creates a test presentation in a temp directory, builds it,
// and returns the built HTML string. Fails the test on any error.
func scaffoldAndBuild(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	slug, err := Create("Timer Test", 4)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	root := filepath.Join(tmp, slug)

	result, err := Build(root)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	return result.HTML
}

// TestBuildContainsTimerFeatures verifies that built presentations include
// the core timer functions (toggleTimer, startTimer, pauseTimer, formatTime,
// getElapsedMs) in the inlined JavaScript output. These functions power the
// elapsed presentation timer in the speaker notes window.
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

// TestBuildContainsNotesTimerUI verifies that the speaker notes window HTML
// builder includes timer display elements: the elapsed time display
// (notesTimer), per-slide reading time (notesReadingTime), remaining deck
// time (notesRemaining), and the start/pause toggle button (notesTimerToggle).
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

// TestBuildTimerKeyboardShortcut verifies that the T key is registered as a
// keyboard shortcut for toggling the presentation timer. The handler should
// call toggleTimer() on both 't' and 'T' key presses.
func TestBuildTimerKeyboardShortcut(t *testing.T) {
	html := scaffoldAndBuild(t)

	if !strings.Contains(html, "e.key === 't'") && !strings.Contains(html, `e.key === 'T'`) {
		t.Error("built HTML missing T key handler for timer toggle")
	}
}

// TestBuildReadingTimeComputation verifies that the reading time computation
// function (computeReadingTimes) is present in the built output. This function
// calculates per-slide word counts at page load, excluding speaker notes
// content, and converts to estimated reading time at 200 words per minute.
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
