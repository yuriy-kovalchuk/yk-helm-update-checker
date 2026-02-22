package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// DownloadRepo clones a GitHub repository to a local directory.
func DownloadRepo(repoURL, destDir string) error {
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		// If directory exists, try to update it or remove it
		// For simplicity, we'll remove it and re-clone
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Set a 2-minute timeout for cloning
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Use -- to ensure repoURL is treated as a positional argument, not a flag
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--", repoURL, destDir)
	
	// Disable interactive prompts (don't wait for password/username)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git clone timed out")
		}
		return fmt.Errorf("git clone failed: %s, %w", string(output), err)
	}

	return nil
}
