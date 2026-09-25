//go:build !unix

package cmd

import "os/exec"

// isolateProcessGroup is a no-op off Unix: cancellation kills only the
// analyzer process itself, not anything it started.
func isolateProcessGroup(cmd *exec.Cmd) {}
