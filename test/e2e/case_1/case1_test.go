package case1_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGoMod(t *testing.T, dir string) {
	goModContent := `module case1
go 1.22.4

replace github.com/robo-monk/lid => ../../../../
require github.com/robo-monk/lid v0.0.0
`
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
}

func getProcessList(t *testing.T) string {
	output := runCmd(t, "./case1", "list")
	fmt.Println("\n output: ", output)
	return output
}

type ProcessInfo struct {
	Status string
	Uptime string
	PID    int
	CPU    string
	Memory string
}

func TrimAnsi(s string) string {
	ansiRegex := regexp.MustCompile("\x1b\\[[0-9;]*[mGKH]")
	return ansiRegex.ReplaceAllString(s, "")
}

func GetProcessInfoByName(output, procName string) (*ProcessInfo, error) {
	// This regex attempts to match a line structured as:
	// │ Name │ Status │ Uptime │ PID │ CPU │ Memory │
	// Each column is captured into a named group. We are using (?P<...>) syntax for named groups.
	regexPattern := `^\s*│\s*(?P<Name>[^│]+)\s*│\s*(?P<Status>[^│]+)\s*│\s*(?P<Uptime>[^│]+)\s*│\s*(?P<PID>[^│]+)\s*│\s*(?P<CPU>[^│]+)\s*│\s*(?P<Memory>[^│]+)\s*│\s*$`
	re := regexp.MustCompile(regexPattern)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		// Extract the indices for named captures
		nameIdx := re.SubexpIndex("Name")
		statusIdx := re.SubexpIndex("Status")
		uptimeIdx := re.SubexpIndex("Uptime")
		pidIdx := re.SubexpIndex("PID")
		cpuIdx := re.SubexpIndex("CPU")
		memoryIdx := re.SubexpIndex("Memory")

		name := strings.TrimSpace(match[nameIdx])
		if name == procName {
			pid := strings.TrimSpace(match[pidIdx])
			pidInt, _ := strconv.Atoi(pid)

			return &ProcessInfo{
				Status: TrimAnsi(strings.TrimSpace(match[statusIdx])),
				Uptime: TrimAnsi(strings.TrimSpace(match[uptimeIdx])),
				PID:    pidInt,
				CPU:    TrimAnsi(strings.TrimSpace(match[cpuIdx])),
				Memory: TrimAnsi(strings.TrimSpace(match[memoryIdx])),
			}, nil
		}
	}

	return nil, fmt.Errorf("process %q not found", procName)
}

func AssertProcessStatus(t *testing.T, processName string, status string) {
	output := getProcessList(t)
	processInfo, err := GetProcessInfoByName(output, processName)
	require.NoError(t, err)
	assert.Equal(t, status, processInfo.Status)
}
func RequireProcessStatus(t *testing.T, processName string, status string) {
	output := getProcessList(t)
	processInfo, err := GetProcessInfoByName(output, processName)
	require.NoError(t, err)
	require.Equal(t, status, processInfo.Status)
}

func CaptureRunningProcess(t *testing.T, processName string) *process.Process {
	output := getProcessList(t)
	processInfo, err := GetProcessInfoByName(output, processName)
	require.NoError(t, err)
	require.Equal(t, "Running", processInfo.Status)
	process, err := process.NewProcess(int32(processInfo.PID))
	require.NoError(t, err)
	isRunning, err := process.IsRunning()
	require.NoError(t, err)
	require.True(t, isRunning)
	return process
}

func runCmd(t *testing.T, name string, args ...string) string {
	testdataDir := filepath.Join("testdata")
	cmd := exec.Command(name, args...)
	outputBuffer := bytes.NewBuffer(nil)
	cmd.Stdout = io.MultiWriter(os.Stdout, outputBuffer)
	cmd.Stderr = io.MultiWriter(os.Stderr, outputBuffer)
	cmd.Dir = testdataDir
	err := cmd.Run()
	require.NoError(t, err)
	return outputBuffer.String()
}

func TestCase1(t *testing.T) {
	// Build the test application
	testdataDir := filepath.Join("testdata")

	// Ensure cleanup
	defer func() {

		runCmd(t, "./case1", "stop")

		os.Remove(filepath.Join(testdataDir, "case1"))
		os.Remove(filepath.Join(testdataDir, "lid.log"))
		os.Remove(filepath.Join(testdataDir, "go.mod"))
		os.Remove(filepath.Join(testdataDir, "go.sum"))
	}()

	setupGoMod(t, testdataDir)
	runCmd(t, "go", "mod", "tidy")
	runCmd(t, "go", "build", "-o", "case1")

	RequireProcessStatus(t, "worker", "Stopped")

	go runCmd(t, "./case1", "start", "worker")

	time.Sleep(100 * time.Millisecond)

	AssertProcessStatus(t, "worker", "Starting")
	AssertProcessStatus(t, "unstable-service", "Stopped")

	// Give services time to start
	time.Sleep(500 * time.Millisecond)
	// Test process management
	AssertProcessStatus(t, "worker", "Running")
	AssertProcessStatus(t, "unstable-service", "Stopped")

	// start other process
	runCmd(t, "./case1", "start", "unstable-service")
	AssertProcessStatus(t, "worker", "Running")
	AssertProcessStatus(t, "unstable-service", "Running")

	runCmd(t, "./case1", "stop")
	time.Sleep(500 * time.Millisecond)
	RequireProcessStatus(t, "worker", "Stopped")
	RequireProcessStatus(t, "unstable-service", "Stopped")
	return
}
