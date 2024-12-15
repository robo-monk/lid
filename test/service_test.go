package lid_test

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/robo-monk/lid/lid"
	"github.com/stretchr/testify/assert"
)

type TestService struct {
	t        *testing.T
	chanDone chan struct{}
	*lid.Service
}

func NewTestService(t *testing.T, s lid.ServiceConfig) (*TestService, *lid.Service) {
	ts := &TestService{
		t:        t,
		chanDone: make(chan struct{}),
		Service:  lid.NewService(t.Name(), s),
	}
	return ts, ts.Service
}

func (ts *TestService) Start() {
	ts.Service.Start()
	close(ts.chanDone)
}

func (ts *TestService) WaitOrTimeout(timeout time.Duration) {
	select {
	case <-ts.chanDone:
	case <-time.After(timeout):
		ts.t.Fatal("Test timed out waiting for service to complete")
	}
}

func TestNewService(t *testing.T) {
	_, s := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "-c", "sleep 1; exit 1"},
	})

	assert.True(t, s.GetCachedStatus() == lid.STOPPED || s.GetCachedStatus() == lid.EXITED, s.GetCachedStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())
}

func TestNewServiceStart(t *testing.T) {
	ts, s := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "-c", "sleep 0.1; exit 1"},
	})

	assert.True(t, s.GetCachedStatus() == lid.STOPPED || s.GetCachedStatus() == lid.EXITED, s.GetCachedStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())

	go ts.Start()

	// Wait for the task to run
	time.Sleep(10 * time.Millisecond)

	// Task should be running right now
	assert.Equal(t, lid.RUNNING, s.GetCachedStatus(), "Process should be RUNNING")
	assert.NotEqual(t, lid.NO_PID, s.GetPid(), "PID should not be 0")

	ts.WaitOrTimeout(1 * time.Second)

	// Task should be done
	assert.Equal(t, lid.EXITED, s.GetCachedStatus(), "Process should be exited")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")
}

func TestNewServiceStartStop(t *testing.T) {
	ts, s := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "./e2e/mock_services/long_running.sh"},
	})

	assert.True(t, s.GetCachedStatus() == lid.STOPPED || s.GetCachedStatus() == lid.EXITED, s.GetCachedStatus())
	assert.Equal(t, lid.NO_PID, s.GetPid())

	go ts.Start()

	start := time.Now().UnixMilli()
	time.Sleep(10 * time.Millisecond)

	// Task should be running right now
	assert.Equal(t, lid.RUNNING, s.GetCachedStatus(), "Process should be RUNNING")
	assert.NotEqual(t, lid.NO_PID, s.GetPid(), "PID should not be 0")

	fmt.Printf("stopping service\n")
	s.Stop()
	fmt.Printf("service stopped\n")

	assert.Equal(t, lid.STOPPED, s.GetCachedStatus(), "Process should be STOPPED, since we stopped the task manually")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")

	ts.WaitOrTimeout(1 * time.Second)

	assert.Less(t, time.Now().UnixMilli()-start, int64(200), "Process should not be allowed to complete")
	assert.Equal(t, lid.STOPPED, s.GetCachedStatus(), "Process should be STOPPED")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")
}

func TestNewServiceOnExit(t *testing.T) {
	recievedErrorCode := -1

	ts, _ := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "-c", "sleep 0; exit 32"},
		OnExit: func(e *exec.ExitError, self *lid.Service) {
			recievedErrorCode = e.ExitCode()
		},
	})

	assert.True(t, ts.GetCachedStatus() == lid.STOPPED || ts.GetCachedStatus() == lid.EXITED, ts.GetCachedStatus())
	assert.Equal(t, lid.NO_PID, ts.GetPid())
	ts.Start()

	// Task should be running right now
	assert.Equal(t, lid.EXITED, ts.GetCachedStatus(), "Process should be EXITED")
	assert.Equal(t, recievedErrorCode, 32, "Exit codes do not match")
}

func TestNewServiceOnExitDoesNotRunWhenStopped(t *testing.T) {
	recievedErrorCode := -1

	ts, s := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "./e2e/mock_services/long_running.sh"},
		OnExit: func(e *exec.ExitError, self *lid.Service) {
			recievedErrorCode = e.ExitCode()
		},
	})

	start := time.Now().UnixMilli()

	go ts.Start()

	// Wait for the task to start running
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, lid.RUNNING, s.GetCachedStatus(), "Process should be RUNNING")
	assert.NotEqual(t, lid.NO_PID, s.GetPid(), "PID should not be 0")

	s.Stop()

	ts.WaitOrTimeout(1 * time.Second)

	assert.Equal(t, lid.STOPPED, s.GetCachedStatus(), "Process should be STOPPED, since we stopped the task manually")
	assert.Equal(t, lid.NO_PID, s.GetPid(), "PID should be 0")
	assert.Less(t, time.Now().UnixMilli()-start, int64(100), "Process should not be allowed to complete")
	assert.Equal(t, recievedErrorCode, -1, "OnExit function should not have run")
}

func TestNewServiceStartStart(t *testing.T) {
	recievedErrorCode := -1

	ts, s := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "./e2e/mock_services/long_running.sh"},
		OnExit: func(e *exec.ExitError, self *lid.Service) {
			recievedErrorCode = e.ExitCode()
		},
	})

	start := time.Now().UnixMilli()

	go ts.Start()

	// Wait for the task to start running
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, lid.RUNNING, s.GetCachedStatus(), "Process should be RUNNING")
	currentPid := s.GetPid()
	assert.NotEqual(t, lid.NO_PID, currentPid, "PID should not be 0")

	err := s.Start()

	assert.ErrorIs(t, err, lid.ErrProcessAlreadyRunning)
	assert.Equal(t, lid.RUNNING, s.GetCachedStatus(), "Process should be still RUNNING")
	assert.Equal(t, currentPid, s.GetPid(), "PID should be the same")

	s.Stop()
	ts.WaitOrTimeout(1 * time.Second)

	assert.Less(t, time.Now().UnixMilli()-start, int64(100), "Process should not be allowed to complete")
	assert.Equal(t, recievedErrorCode, -1, "OnExit function should not have run")
}

func TestOnBeforeStart(t *testing.T) {
	beforeStartCalled := false
	shouldPreventStart := true

	ts, s := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "-c", "sleep 0.5; exit 1"},
		OnBeforeStart: func(self *lid.Service) error {
			beforeStartCalled = true
			if shouldPreventStart {
				return fmt.Errorf("preventing start")
			}
			return nil
		},
	})

	// First attempt - OnBeforeStart should prevent service from starting
	err := s.Start()
	assert.NotNil(t, err)
	assert.Equal(t, "preventing start", err.Error())
	assert.True(t, beforeStartCalled)

	assert.True(t, s.GetCachedStatus() == lid.STOPPED || s.GetCachedStatus() == lid.EXITED)
	assert.Equal(t, lid.NO_PID, s.GetPid())

	// Reset and allow start
	beforeStartCalled = false
	shouldPreventStart = false

	go ts.Start()

	time.Sleep(50 * time.Millisecond)
	assert.True(t, beforeStartCalled)
	assert.Equal(t, lid.RUNNING, s.GetCachedStatus())
	assert.NotEqual(t, lid.NO_PID, s.GetPid())

	s.Stop()
	ts.WaitOrTimeout(1 * time.Second)
}

func TestOnAfterStart(t *testing.T) {
	afterStartCalled := false
	var capturedPid int32

	ts, s := NewTestService(t, lid.ServiceConfig{
		Command: []string{"bash", "./e2e/mock_services/long_running.sh"},
		OnAfterStart: func(self *lid.Service) {
			afterStartCalled = true
			capturedPid = self.GetPid()
		},
	})

	go ts.Start()

	time.Sleep(10 * time.Millisecond)
	assert.True(t, afterStartCalled)
	assert.Equal(t, lid.RUNNING, s.GetCachedStatus())
	assert.NotEqual(t, lid.NO_PID, capturedPid)
	assert.Equal(t, capturedPid, s.GetPid())

	s.Stop()
	ts.WaitOrTimeout(1 * time.Second)
}
