package lid_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/robo-monk/lid/lid"
	"github.com/stretchr/testify/assert"
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

func TestNewService(t *testing.T) {
	// t.Parallel()
	s := lid.Service{
		Logger: log.New(os.Stdout, "[TEST] ", 0),
		Name:   "test-process",
		// Cwd:     "",
		Command: []string{"bash", "-c", "sleep 1; exit 1"},
		// EnvFile: "",
	}

	assert.True(t, s.GetStatus() == lid.STOPPED || s.GetStatus() == lid.EXITED, s.GetStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())
}

func TestNewServiceStart(t *testing.T) {
	s := lid.Service{
		Logger: log.New(os.Stdout, "[TEST] ", 0),
		Name:   "test-process",
		// Cwd:     "",
		Command: []string{"bash", "-c", "sleep 0.1; exit 1"},
		// EnvFile: "",
	}

	assert.True(t, s.GetStatus() == lid.STOPPED || s.GetStatus() == lid.EXITED, s.GetStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
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

func TestNewServiceStartStop(t *testing.T) {
	s := lid.Service{
		Logger:  log.New(os.Stdout, "[TEST] ", 0),
		Name:    "test-process",
		Command: []string{"bash", "-c", "sleep 5; exit 0"},
	}

	assert.True(t, s.GetStatus() == lid.STOPPED || s.GetStatus() == lid.EXITED, s.GetStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		s.Start()
	}()

	start := time.Now().UnixMilli()
	// Wait for the task to start runnng
	time.Sleep(10 * time.Millisecond)

	// Task should be running right now
	assert.Equal(t, lid.RUNNING, s.GetStatus(), "Process should be RUNNING")
	assert.NotEqual(t, lid.NO_PID, s.GetPid(), "PID should not be 0")

	s.Stop()

	assert.Equal(t, lid.STOPPED, s.GetStatus(), "Process should be STOPPED, since we stopped the task manually")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")

	wg.Wait()

	assert.Less(t, time.Now().UnixMilli()-start, int64(100), "Process should not be allowed to complete")
	assert.Equal(t, lid.STOPPED, s.GetStatus(), "Process should be STOPPED")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")
}

func TestNewServiceOnExit(t *testing.T) {
	recievedErrorCode := -1

	s := lid.Service{
		Logger:  log.New(os.Stdout, "[TEST] ", 0),
		Name:    "test-process",
		Command: []string{"bash", "-c", "sleep 0; exit 32"},
		OnExit: func(e *exec.ExitError, self *lid.Service) {
			recievedErrorCode = e.ExitCode()
		},
	}

	assert.True(t, s.GetStatus() == lid.STOPPED || s.GetStatus() == lid.EXITED, s.GetStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())
	s.Start()

	// Task should be running right now
	assert.Equal(t, lid.EXITED, s.GetStatus(), "Process should be EXITED")
	assert.Equal(t, recievedErrorCode, 32, "Exit codes do not match")
}

func TestNewServiceOnExitDoesNotRunWhenStopped(t *testing.T) {
	recievedErrorCode := -1

	s := lid.Service{
		Logger:  log.New(os.Stdout, "[TEST] ", 0),
		Name:    "test-process",
		Command: []string{"bash", "-c", "sleep 5; exit 32"},
		OnExit: func(e *exec.ExitError, self *lid.Service) {
			recievedErrorCode = e.ExitCode()
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		s.Start()
	}()

	start := time.Now().UnixMilli()

	// Wait for the task to start runnng
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, lid.RUNNING, s.GetStatus(), "Process should be RUNNING")
	assert.NotEqual(t, lid.NO_PID, s.GetPid(), "PID should not be 0")

	s.Stop()

	assert.Equal(t, lid.STOPPED, s.GetStatus(), "Process should be STOPPED, since we stopped the task manually")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")

	wg.Wait()

	assert.Less(t, time.Now().UnixMilli()-start, int64(100), "Process should not be allowed to complete")
	assert.Equal(t, recievedErrorCode, -1, "OnExit function should not have run")
}

func TestNewServiceStartStart(t *testing.T) {
	recievedErrorCode := -1

	s := lid.Service{
		Logger:  log.New(os.Stdout, "[TEST] ", 0),
		Name:    "test-process",
		Command: []string{"bash", "-c", "sleep 1; exit 32"},
		OnExit: func(e *exec.ExitError, self *lid.Service) {
			recievedErrorCode = e.ExitCode()
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := s.Start()
		assert.Nil(t, err)
	}()

	start := time.Now().UnixMilli()

	// Wait for the task to start runnng
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, lid.RUNNING, s.GetStatus(), "Process should be RUNNING")
	currentPid := s.GetPid()
	assert.NotEqual(t, lid.NO_PID, currentPid, "PID should not be 0")

	err := s.Start()

	// assert.NotNil(t, err)
	// assert.Equal(t, err.Error(), "service 'test-process' is already running")
	assert.ErrorIs(t, err, lid.ErrProcessAlreadyRunning)
	assert.Equal(t, lid.RUNNING, s.GetStatus(), "Process should be still RUNNING")
	assert.Equal(t, currentPid, s.GetPid(), "PID should be the same")
	s.Stop()
	wg.Wait()

	assert.Less(t, time.Now().UnixMilli()-start, int64(100), "Process should not be allowed to complete")
	assert.Equal(t, recievedErrorCode, -1, "OnExit function should not have run")
}

func TestOnBeforeStart(t *testing.T) {
	beforeStartCalled := false
	shouldPreventStart := true

	s := lid.Service{
		Logger:  log.New(os.Stdout, "[TEST] ", 0),
		Name:    "test-process",
		Command: []string{"bash", "-c", "sleep 0.1; exit 0"},
		OnBeforeStart: func(self *lid.Service) error {
			beforeStartCalled = true
			if shouldPreventStart {
				return fmt.Errorf("preventing start")
			}
			return nil
		},
	}

	// First attempt - OnBeforeStart should prevent service from starting
	err := s.Start()
	assert.NotNil(t, err)
	assert.Equal(t, "preventing start", err.Error())
	assert.True(t, beforeStartCalled)
	assert.True(t, s.GetStatus() == lid.STOPPED || s.GetStatus() == lid.EXITED)
	assert.Equal(t, lid.NO_PID, s.GetPid())

	// Reset and allow start
	beforeStartCalled = false
	shouldPreventStart = false

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := s.Start()
		assert.Nil(t, err)
	}()

	time.Sleep(10 * time.Millisecond)
	assert.True(t, beforeStartCalled)
	assert.Equal(t, lid.RUNNING, s.GetStatus())
	assert.NotEqual(t, lid.NO_PID, s.GetPid())

	s.Stop()
	wg.Wait()
}

func TestOnAfterStart(t *testing.T) {
	afterStartCalled := false
	var capturedPid int32

	s := lid.Service{
		Logger:  log.New(os.Stdout, "[TEST] ", 0),
		Name:    "test-process",
		Command: []string{"bash", "-c", "sleep 0.1; exit 0"},
		OnAfterStart: func(self *lid.Service) {
			afterStartCalled = true
			capturedPid = self.GetPid()
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := s.Start()
		assert.Nil(t, err)
	}()

	time.Sleep(10 * time.Millisecond)
	assert.True(t, afterStartCalled)
	assert.Equal(t, lid.RUNNING, s.GetStatus())
	assert.NotEqual(t, lid.NO_PID, capturedPid)
	assert.Equal(t, capturedPid, s.GetPid())

	s.Stop()
	wg.Wait()
}
