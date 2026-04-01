package cmd

import "testing"

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		cur, min string
		want     bool
	}{
		{"0.1.0", "0.0.10", true},
		{"0.0.9", "0.0.10", false},
		{"0.0.10", "0.0.10", true},
		{"v1.2.3", "1.2.3", true},
		{"1.2.4", "1.2.3", true},
	}
	for _, tc := range tests {
		if got := versionAtLeast(tc.cur, tc.min); got != tc.want {
			t.Errorf("versionAtLeast(%q, %q) = %v, want %v", tc.cur, tc.min, got, tc.want)
		}
	}
}
