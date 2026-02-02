package qmldiff

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/rmitchellscott/rm-qmd-hasher/internal/logging"
)

type Service struct {
	binaryPath string
}

func NewService(binaryPath string) *Service {
	return &Service{
		binaryPath: binaryPath,
	}
}

func (s *Service) HashDiffs(hashtabPath, qmdPath string) error {
	cmd := exec.Command(s.binaryPath, "hash-diffs", hashtabPath, qmdPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logging.Debug(logging.ComponentQMLDiff, "Running: %s hash-diffs %s %s", s.binaryPath, hashtabPath, qmdPath)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("hash-diffs failed: %w\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	return nil
}

func (s *Service) GetBinaryPath() string {
	return s.binaryPath
}
