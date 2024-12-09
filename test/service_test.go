package lid_test

import (
	// "os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/robo-monk/lid/lid"
	"github.com/stretchr/testify/assert"
	// "github.com/shirou/gopsutil/v4/process"
	// "github.com/yourusername/yourproject/lid"
)

// TestServiceProcessReadWrite tests writing and reading a ServiceProcess to/from a file.
func TestServiceProcessReadWrite(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "service-process.lid")

	original := lid.ServiceProcess{
		Status: lid.RUNNING,
		Pid:    1234,
	}

	if err := original.WriteToFile(filename); err != nil {
		t.Fatalf("Failed to write service process: %v", err)
	}

	readBack, err := lid.ReadServiceProcess(filename)
	if err != nil {
		t.Fatalf("Failed to read service process: %v", err)
	}

	if readBack.Status != original.Status || readBack.Pid != original.Pid {
		t.Errorf("Mismatch in service process: got %v, want %v", readBack, original)
	}
}

// TestReadServiceProcessFileNotFound checks behavior when file doesn't exist.
func TestReadServiceProcessFileNotFound(t *testing.T) {
	t.Parallel()
	_, err := lid.ReadServiceProcess("nonexistent-file.lid")

	// assert.Equal(t, int64(0), sp.Pid, "Expect PID to be 0 (inactive)");
	// assert.Equal(t, lid.STOPPED, sp.Status, "Expected Process status to be STOPPED")
	assert.NotNil(t, err, "Expected error not to be nil")
}


// TestReadServiceProcessCorruptFile tests error handling with invalid binary data.
// func TestReadServiceProcessCorruptFile(t *testing.T) {
// 	t.Parallel()
// 	tmpDir := t.TempDir()
// 	filename := filepath.Join(tmpDir, "corrupt.lid")

// 	if err := os.WriteFile(filename, []byte("not a valid struct"), 0644); err != nil {
// 		t.Fatal(err)
// 	}

// 	sp, err := lid.ReadServiceProcess(filename)

// 	assert.Equal(t, int64(0), sp.Pid, "Expect PID to be 0 (inactive)");
// 	assert.Equal(t, lid.STOPPED, sp.Status, "Expected Process status to be STOPPED")
// 	assert.Nil(t, err, "Expected error to be nil")
// }

// TestReadDotEnvFile checks reading well-formed env files.
func TestReadDotEnvFile_Valid(t *testing.T) {
	t.Parallel()
	filename := filepath.Join("testdata", "valid.env")
	env := lid.ReadDotEnvFile(filename)
	expected := []string{"FOO=bar", "BAZ=qux"}

	assert.Equal(t, len(expected), len(env), "Env len mismatch")

	for i, line := range expected {
		assert.Equal(t, line, env[i])
	}
}

// TestReadDotEnvFile_Quoted tests removing quotes.
func TestReadDotEnvFile_Quoted(t *testing.T) {
	t.Parallel()
	filename := filepath.Join("testdata", "quoted.env")
	env := lid.ReadDotEnvFile(filename)

	expected := []string{"FOO=hello world", "BAR= spaced value "}
	assert.Equal(t, len(expected), len(env), "Env len mismatch")
	for i, line := range expected {
		assert.Equal(t, line, env[i])
	}
}

// TestReadDotEnvFile_Malformed checks behavior with malformed env files.
// Here we mainly ensure it doesn't panic and returns partial results.
func TestReadDotEnvFile_Malformed(t *testing.T) {
	t.Parallel()
	filename := filepath.Join("testdata", "malformed.env")
	env := lid.ReadDotEnvFile(filename)
	// We don't define strict behavior for malformed lines, just ensure no crash.
	if len(env) == 0 {
		t.Errorf("Expected some parsed environment variables, got none")
	}
}

func TestNewService(t *testing.T) {
	// t.Parallel()
	m := lid.New();

	s := lid.Service {
		Lid:     m,
		Name:    "test-process",
		// Cwd:     "",
		Command: []string{ "bash", "-c", "sleep 1; exit 1"},
		// EnvFile: "",
	}

	assert.True(t, s.GetStatus() == lid.STOPPED || s.GetStatus() == lid.EXITED, s.GetStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())
}

func TestNewServiceStart(t *testing.T) {
	// t.Parallel()
	m := lid.New();

	s := lid.Service {
		Lid:     m,
		Name:    "test-process",
		// Cwd:     "",
		Command: []string{ "bash", "-c", "sleep 0.1; exit 0"},
		// EnvFile: "",
	}

	assert.True(t, s.GetStatus() == lid.STOPPED || s.GetStatus() == lid.EXITED, s.GetStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())


	var wg sync.WaitGroup
	wg.Add(1)

	go func(){
		defer wg.Done()
		s.Start()
	}()

	// Wait for the task to run
	time.Sleep(10 * time.Millisecond)

	// Task should be running right now
	assert.Equal(t, lid.RUNNING, s.GetStatus(), "Process should be RUNNING")
	assert.NotEqual(t, lid.NO_PID, s.GetPid(), "PID should not be 0")

	wg.Wait()
	// Task should be done
	assert.Equal(t, lid.EXITED, s.GetStatus(), "Process should be exited")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")

}
