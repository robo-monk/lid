package lid

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
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

func contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func tailFile(filePath string, callback func(line string) bool) error {
	// Open the file for reading.
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Seek to the end of the file.
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// Handle EOF: Wait for new data to be written.
			if err.Error() == "EOF" {
				time.Sleep(50 * time.Millisecond) // Polling interval.
				continue
			}
			return fmt.Errorf("error reading file: %w", err)
		}

		// Print the line that was read.
		if callback(line) {
			return nil
		}
	}
}
