package case1_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

func getProcessList(t *testing.T, testdataDir string) string {
	cmd := exec.Command("./case1", "list")
	cmd.Dir = testdataDir
	processes, err := cmd.Output()
	fmt.Println("\n", string(processes))
	assert.NoError(t, err)
	return string(processes)
}

func captureRunningProcess(t *testing.T, output string, processName string) (int, *process.Process) {
	regex := regexp.MustCompile(fmt.Sprintf(`%s.+│.+Running.+│ (\d+) │`, processName))
	match := regex.FindStringSubmatch(output)
	assert.Equal(t, 2, len(match))
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

func TestCase1(t *testing.T) {

	// Build the test application
	testdataDir := filepath.Join("testdata")
	cmd := exec.Command("go", "mod", "init", "case1")
	cmd.Dir = testdataDir
	err := cmd.Run()
	require.NoError(t, err)

	// cmd = exec.Command("go", "mod", "edit", "-replace", "github.com/robo-monk/lid=../../../")
	// cmd.Dir = testdataDir
	// err = cmd.Run()
	// require.NoError(t, err)

	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = testdataDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("go", "build", "-o", "case1")
	cmd.Dir = testdataDir
	err = cmd.Run()
	require.NoError(t, err)

	// Ensure cleanup
	defer func() {
		os.Remove(filepath.Join(testdataDir, "case1"))
		os.Remove(filepath.Join(testdataDir, "lid.log"))
		os.Remove(filepath.Join(testdataDir, "go.mod"))
		os.Remove(filepath.Join(testdataDir, "go.sum"))
	}()

	// Start the application
	app := exec.Command("./case1", "start")
	app.Dir = testdataDir
	err = app.Run()
	require.NoError(t, err)

	// Give services time to start
	time.Sleep(10 * time.Millisecond)

	// Test process management
	output := getProcessList(t, testdataDir)

	_, process := captureRunningProcess(t, output, "unstable-service")
	children, err := process.Children()
	require.NoError(t, err)
	require.Equal(t, 1, len(children))
	children[0].Kill()

	_, process = captureRunningProcess(t, output, "worker")
	children, err = process.Children()
	require.NoError(t, err)
	require.Equal(t, 1, len(children))
	children[0].Kill()

	// output = getProcessList(t, testdataDir)
	// assertStopped(t, output, "unstable-service")
	// assertStopped(t, output, "worker")

	// time.Sleep(100 * time.Millisecond)

	output = getProcessList(t, testdataDir)
	captureRunningProcess(t, output, "unstable-service")
	captureRunningProcess(t, output, "worker")

	cmd = exec.Command("./case1", "stop")
	cmd.Dir = testdataDir
	err = cmd.Run()
	require.NoError(t, err)

	// time.Sleep(100 * time.Millisecond)

	output = getProcessList(t, testdataDir)
	assertStopped(t, output, "unstable-service")
	assertStopped(t, output, "worker")
}
