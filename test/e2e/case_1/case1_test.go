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

func assertStopped(t *testing.T, output string, processName string) {
	regex := regexp.MustCompile(fmt.Sprintf(`%s.+│.+Stopped.+│ (-) `, processName))
	match := regex.FindStringSubmatch(output)
	require.Equal(t, 2, len(match))
}

func getProcessList(t *testing.T) string {
	output := runCmd(t, "./case1", "list")
	fmt.Println("\n output: ", output)
	return output
}

func captureRunningProcess(t *testing.T, output string, processName string) (int, *process.Process) {
	regexExpr := fmt.Sprintf(`%s.+│.+Running.+│ (\d+) │ `, processName)
	fmt.Println("regexExpr", regexExpr)
	regex := regexp.MustCompile(regexExpr)
	match := regex.FindStringSubmatch(output)
	fmt.Println("match", match)
	require.Equal(t, 2, len(match))
	pid, err := strconv.Atoi(match[1]) // parse the pid
	require.NoError(t, err)
	fmt.Printf("Captured PID: %v\n", pid)
	process, err := process.NewProcess(int32(pid))
	require.NoError(t, err)
	isRunning, err := process.IsRunning()
	require.NoError(t, err)
	require.True(t, isRunning)
	return pid, process
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
		// os.Remove(filepath.Join(testdataDir, "case1"))
		os.Remove(filepath.Join(testdataDir, "lid.log"))
		os.Remove(filepath.Join(testdataDir, "go.mod"))
		os.Remove(filepath.Join(testdataDir, "go.sum"))
	}()

	runCmd(t, "go", "mod", "init", "case1")
	runCmd(t, "go", "mod", "tidy")
	runCmd(t, "go", "build", "-o", "case1")
	runCmd(t, "./case1", "start")

	AssertProcessStatus(t, "unstable-service", "Starting")
	// Give services time to start
	time.Sleep(500 * time.Millisecond)

	// Test process management
	AssertProcessStatus(t, "unstable-service", "Running")
	// AssertProcessStatus(t, "worker", "Running")

	return
	process := CaptureRunningProcess(t, "unstable-service")
	children, err := process.Children()
	require.NoError(t, err)
	require.Equal(t, 1, len(children))
	children[0].Kill()

	AssertProcessStatus(t, "unstable-service", "Stopped")
	AssertProcessStatus(t, "worker", "Running")

	process = CaptureRunningProcess(t, "worker")
	children, err = process.Children()
	require.NoError(t, err)
	require.Equal(t, 1, len(children))
	children[0].Kill()

	AssertProcessStatus(t, "worker", "Stopped")

	// _, process := captureRunningProcess(t, output, "unstable-service")
	// children, err := process.Children()
	// require.NoError(t, err)
	// require.Equal(t, 1, len(children))
	// children[0].Kill()

	// _, process = captureRunningProcess(t, output, "worker")
	// children, err = process.Children()
	// require.NoError(t, err)
	// require.Equal(t, 1, len(children))
	// children[0].Kill()

	// pid, err := GetPIDByName(output, "unstable-service")
	require.NoError(t, err)
	// fmt.Println("pid", pid)
	return

	// output = getProcessList(t)
	// assertStopped(t, output, "unstable-service")
	// assertStopped(t, output, "worker")

	// time.Sleep(100 * time.Millisecond)

	// output = getProcessList(t)
	// captureRunningProcess(t, output, "unstable-service")
	// captureRunningProcess(t, output, "worker")

	// runCmd(t, "./case1", "stop")

	// time.Sleep(2000 * time.Millisecond)

	// output = getProcessList(t)
	// assertStopped(t, output, "unstable-service")
	// assertStopped(t, output, "worker")
}
