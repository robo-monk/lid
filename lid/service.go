package lid

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

const NO_PID int32 = 0

const READINESS_CHECK_PASSED_MESSAGE = "Readiness check passed"
const READINESS_CHECK_FAILED_MESSAGE = "Readiness check failed"
const NO_READINESS_CHECK_MESSAGE = "No readiness check, assuming success"

type ServiceStatus int8

const (
	STOPPED ServiceStatus = iota
	EXITED
	STARTING
	RUNNING
	STOPPING
)

func (s ServiceStatus) String() string {
	switch s {
	case STOPPED:
		return "Stopped"
	case EXITED:
		return "Exited"
	case STARTING:
		return "Starting"
	case RUNNING:
		return "Running"
	case STOPPING:
		return "Stopping"
	default:
		return "Unknown"
	}
}

type ServiceProcess struct {
	Status ServiceStatus
	Pid    int32
}

func (sp ServiceProcess) WriteToFile(filename string) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, sp)

	if err != nil {
		return err
	}

	return os.WriteFile(filename, buf.Bytes(), 0666)
}

func ReadServiceProcess(filename string) (ServiceProcess, error) {
	var data ServiceProcess

	file, err := os.Open(filename)

	if err != nil {
		return data, err
	}

	err = binary.Read(file, binary.LittleEndian, &data)
	return data, err
}

type Service struct {
	mu sync.RWMutex

	Logger *log.Logger
	Name   string
	Cwd    string

	Command []string

	EnvFile string
	Env     []string

	GracefulShutdownTimeout time.Duration
	ReadinessCheckTimeout   time.Duration

	Stdout io.Writer
	Stderr io.Writer

	StdoutReadinessCheck func(line string) bool
	OnBeforeStart        func(self *Service) error
	OnAfterStart         func(self *Service)
	OnExit               func(e *exec.ExitError, self *Service)

	ExitSignal syscall.Signal
}

// ServiceConfig defines how a service should be run and managed.
// It provides configuration for things like:
// - Where and how to run the service
// - Environment setup
// - Timeouts and lifecycle hooks
// - Logging and I/O handling
type ServiceConfig struct {
	// Where to run the service from
	Cwd string

	// The actual command to execute (e.g. ["node", "server.js"])
	Command []string

	// Environment configuration
	EnvFile string   // Path to a .env file
	Env     []string // Additional environment variables (overrides EnvFile)

	// Timing configurations
	GracefulShutdownTimeout time.Duration // How long to wait for graceful shutdown
	ReadinessCheckTimeout   time.Duration // How long to wait for service to be ready

	// Where to send service output
	Stdout io.Writer // Service's stdout destination
	Stderr io.Writer // Service's stderr destination
	Logger *log.Logger

	// Lifecycle hooks
	StdoutReadinessCheck func(line string) bool                 // Check service output to determine if it's ready
	OnBeforeStart        func(self *Service) error              // Called just before service starts
	OnAfterStart         func(self *Service)                    // Called right after service starts
	OnExit               func(e *exec.ExitError, self *Service) // Called when service exits

	// What signal to send when stopping the service (defaults to SIGTERM)
	ExitSignal syscall.Signal
}

func NewService(name string, config ServiceConfig) *Service {
	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, fmt.Sprintf("[%s] ", name), log.Ldate|log.Ltime)
	}

	if config.GracefulShutdownTimeout == 0 {
		config.GracefulShutdownTimeout = 5 * time.Second
	}

	if config.ReadinessCheckTimeout == 0 {
		config.ReadinessCheckTimeout = 5 * time.Second
	}

	if config.Stdout == nil {
		config.Stdout = config.Logger.Writer()
	}

	if config.Stderr == nil {
		config.Stderr = config.Logger.Writer()
	}

	if config.ExitSignal == 0 {
		config.ExitSignal = syscall.SIGTERM
	}

	if config.Env == nil {
		config.Env = []string{}
	}

	service := &Service{
		mu:                      sync.RWMutex{},
		Name:                    name,
		Cwd:                     config.Cwd,
		Command:                 config.Command,
		EnvFile:                 config.EnvFile,
		Env:                     config.Env,
		GracefulShutdownTimeout: config.GracefulShutdownTimeout,
		ReadinessCheckTimeout:   config.ReadinessCheckTimeout,
		StdoutReadinessCheck:    config.StdoutReadinessCheck,
		OnBeforeStart:           config.OnBeforeStart,
		OnAfterStart:            config.OnAfterStart,
		OnExit:                  config.OnExit,
		Stdout:                  config.Stdout,
		Stderr:                  config.Stderr,
		Logger:                  config.Logger,
		ExitSignal:              config.ExitSignal,
	}

	return service
}

func (s *Service) GetServiceProcessFilename() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("service-%s.lid", s.Name))
}

func (s *Service) getCachedProcessState() ServiceProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sp, error := ReadServiceProcess(s.GetServiceProcessFilename())
	if error != nil {
		return ServiceProcess{
			Pid:    NO_PID,
			Status: STOPPED,
		}
	}
	return sp
}

func (s *Service) WriteServiceProcess(sp ServiceProcess) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sp.WriteToFile(s.GetServiceProcessFilename())
}

func (s *Service) GetRunningProcess() (*process.Process, error) {
	state := s.getCachedProcessState()
	proc, err := process.NewProcess(int32(state.Pid))

	if err != nil {
		return nil, err
	}

	running, err := proc.IsRunning()

	if !running || err != nil {
		return nil, err
	}

	return proc, nil
}

func (s *Service) IsRunning() bool {
	proc, err := s.GetRunningProcess()
	if err != nil {
		return false
	}

	isRunning, err := proc.IsRunning()

	if err != nil {
		return false
	}

	return isRunning
}

func (s *Service) GetPid() int32 {
	return s.getCachedProcessState().Pid
}

func (s *Service) PrepareCommand() (*exec.Cmd, error) {

	if s.IsRunning() {
		return nil, ErrProcessAlreadyRunning
	}

	cmd := exec.Command(s.Command[0], s.Command[1:]...)

	if s.Cwd != "" {
		cmd.Dir, _ = getRelativePath(s.Cwd)
	}

	cmd.Env = os.Environ()
	if s.EnvFile != "" {
		envPath, _ := getRelativePath(filepath.Join(s.Cwd, s.EnvFile))
		userDefinedEnv, err := ReadDotEnvFile(envPath)

		if err != nil {
			return nil, err
		}

		cmd.Env = append(cmd.Env, userDefinedEnv...)
	}

	// cmd.Env = append(cmd.Env, s.Env...)

	s.Logger.Println("Starting")

	return cmd, nil
}

// func (s *Service) handleReadinessCheck(reader io.ReadCloser, pid int32) error {
func (s *Service) handleReadinessCheck(reader io.Reader, pid int32) error {
	readinessDone := make(chan bool)

	if s.StdoutReadinessCheck != nil {
		s.Logger.Println("Waiting for readiness check")
		s.WriteServiceProcess(ServiceProcess{
			Status: STARTING,
			Pid:    pid,
		})

		go func() {
			defer close(readinessDone)
			scanner := bufio.NewScanner(reader)

			readinessCheckPassed := false
			for scanner.Scan() {
				bytes := scanner.Bytes()

				s.Stdout.Write(bytes)
				s.Stdout.Write([]byte("\n"))

				if s.StdoutReadinessCheck(string(bytes)) {
					readinessCheckPassed = true
					break
				}
			}

			if readinessCheckPassed {
				s.Logger.Println(READINESS_CHECK_PASSED_MESSAGE)
				s.WriteServiceProcess(ServiceProcess{
					Status: RUNNING,
					Pid:    pid,
				})
			} else {
				s.Logger.Println(READINESS_CHECK_FAILED_MESSAGE)
			}
		}()
	} else {
		s.Logger.Println(NO_READINESS_CHECK_MESSAGE)
		s.WriteServiceProcess(ServiceProcess{
			Status: RUNNING,
			Pid:    pid,
		})
		close(readinessDone)
	}

	select {
	case <-readinessDone:
		return nil
	case <-time.After(s.ReadinessCheckTimeout):
		s.Logger.Println("Readiness check timed out")
		s.Stop()
		return fmt.Errorf("readiness check timed out")
	}
}

func (s *Service) Start() error {
	cmd, err := s.PrepareCommand()
	if err != nil {
		s.Logger.Printf("%v\n", err)
		return err
	}

	readerStdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Logger.Printf("%v\n", err)
		return err
	}

	readerStderr, err := cmd.StderrPipe()

	if err != nil {
		s.Logger.Printf("%v\n", err)
		return err
	}

	reader := io.MultiReader(readerStdout, readerStderr)
	s.Logger.Printf("Running Command: %v\n", cmd)

	if s.OnBeforeStart != nil {
		if err := s.OnBeforeStart(s); err != nil {
			s.Logger.Printf("Rejected start: %v\n", err)
			return err
		}
	}

	if err := cmd.Start(); err != nil {
		err = fmt.Errorf("failed to start command: %v", err)
		s.Logger.Printf("%v\n", err)
		return err
	}

	s.Logger.Printf("Started with PID: %d", cmd.Process.Pid)

	if err := s.handleReadinessCheck(reader, int32(cmd.Process.Pid)); err != nil {
		return err
	}

	// go io.Copy(s.Stdout, reader)
	go io.Copy(s.Logger.Writer(), reader)

	if s.OnAfterStart != nil {
		s.OnAfterStart(s)
	}

	s.Logger.Println("Waiting for process to exit")
	err = cmd.Wait()
	s.handleProcessExit(err)
	return nil
}

func (s *Service) handleProcessExit(err error) {
	if err != nil {
		s.Logger.Printf("%v\n", err)
	}

	if s.getCachedProcessState().Status != STOPPED {
		if err != nil {
			s.Logger.Printf("Exited: %v\n", err)
		} else {
			s.Logger.Println("Exited with no error")
		}

		s.WriteServiceProcess(ServiceProcess{
			Status: EXITED,
			Pid:    NO_PID,
		})

		if s.OnExit != nil {
			if err != nil {
				s.OnExit(err.(*exec.ExitError), s)
			} else {
				s.OnExit(&exec.ExitError{}, s)
			}
		}
	} else {
		s.Logger.Println("Stopped")
	}
}

func (s *Service) Stop() error {
	defer func() {
		s.WriteServiceProcess(ServiceProcess{
			Status: STOPPED,
			Pid:    NO_PID,
		})
	}()

	proc, err := s.GetRunningProcess()
	if err != nil || proc == nil {
		return fmt.Errorf("service already down")
	}

	running, err := proc.IsRunning()

	if err == nil && !running {
		return fmt.Errorf("service already down")
	}

	s.Logger.Println("Stopping service")
	s.WriteServiceProcess(ServiceProcess{
		Status: STOPPING,
		Pid:    int32(proc.Pid),
	})

	if err := proc.SendSignal(s.ExitSignal); err != nil {
		s.Logger.Printf("Signal error: %v, using SIGKILL", err)
		proc.Kill()
	}

	terminated := make(chan bool)
	go func() {
		for {
			running, err := proc.IsRunning()
			if err != nil || !running {
				close(terminated)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	select {
	case <-terminated:
		return nil
	case <-time.After(s.GracefulShutdownTimeout):
		s.Logger.Println("Graceful shutdown timeout. Attempting to kill process")
		if err := proc.Kill(); err != nil {
			s.Logger.Printf("Failed to kill process: %v\n", err)
		}
		return fmt.Errorf("failed to terminate service: timeout")
	}
}

func (s *Service) GetCachedStatus() ServiceStatus {
	ps := s.getCachedProcessState()
	return ps.Status
}
