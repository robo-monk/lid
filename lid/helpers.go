package lid

import (
	"fmt"
	"os"
	"path/filepath"
)

func getExecutableDir() (string, error) {
	// Get the path to the executable
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get the directory of the executable
	execDir := filepath.Dir(execPath)
	return execDir, nil
}

func getRelativePath(relativePath string) (string, error) {
	// Get the executable's directory
	execDir, err := getExecutableDir()
	if err != nil {
		return "", err
	}

	// Join the executable's directory with the relative path
	fullPath := filepath.Join(execDir, relativePath)
	return fullPath, nil
}
