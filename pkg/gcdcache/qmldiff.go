package gcdcache

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/rmitchellscott/rm-qmd-hasher/internal/logging"
)

func runQmldiff(binary string, args ...string) error {
	cmd := exec.Command(binary, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logging.Debug(logging.ComponentGCD, "Running: %s %v", binary, args)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %w\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	return nil
}
