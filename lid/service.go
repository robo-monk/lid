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
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

const NO_PID int32 = 0

type ServiceStatus int8

const READINESS_CHECK_PASSED_MESSAGE = "Readiness check passed"
const READINESS_CHECK_FAILED_MESSAGE = "Readiness check failed"
const NO_READINESS_CHECK_MESSAGE = "No readiness check, assuming success"

const (
	STOPPED ServiceStatus = iota
	EXITED
	STARTING
	RUNNING
	STOPPING
)

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
	Logger *log.Logger
	Name   string
	Cwd    string

	Command []string
	EnvFile string

	GracefulShutdownTimeout time.Duration
	ReadinessCheckTimeout   time.Duration

	Stdout io.Writer
	Stderr io.Writer

	StdoutReadinessCheck func(line string) bool
	OnBeforeStart        func(self *Service) error
	OnAfterStart         func(self *Service)
	OnExit               func(e *exec.ExitError, self *Service)
}

type ServiceConfig struct {
	Cwd     string
	Command []string
	EnvFile string

	GracefulShutdownTimeout time.Duration
	ReadinessCheckTimeout   time.Duration

	Stdout io.Writer
	Stderr io.Writer

	Logger               *log.Logger
	StdoutReadinessCheck func(line string) bool
	OnBeforeStart        func(self *Service) error
	OnAfterStart         func(self *Service)
	OnExit               func(e *exec.ExitError, self *Service)
}

func NewService(name string, config ServiceConfig) *Service {
	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, fmt.Sprintf("[%s] ", name), log.Ldate|log.Ltime)
	}

	if config.GracefulShutdownTimeout == 0 {
		config.GracefulShutdownTimeout = 1 * time.Second
	}

	if config.ReadinessCheckTimeout == 0 {
		config.ReadinessCheckTimeout = 1 * time.Second
	}

	if config.Stdout == nil {
		config.Stdout = config.Logger.Writer()
	}

	if config.Stderr == nil {
		config.Stderr = config.Logger.Writer()
	}

	service := &Service{
		Name:                    name,
		Cwd:                     config.Cwd,
		Command:                 config.Command,
		EnvFile:                 config.EnvFile,
		GracefulShutdownTimeout: config.GracefulShutdownTimeout,
		ReadinessCheckTimeout:   config.ReadinessCheckTimeout,
		StdoutReadinessCheck:    config.StdoutReadinessCheck,
		OnBeforeStart:           config.OnBeforeStart,
		OnAfterStart:            config.OnAfterStart,
		OnExit:                  config.OnExit,
		Stdout:                  config.Stdout,
		Stderr:                  config.Stderr,
		Logger:                  config.Logger,
	}

	p, err := service.GetRunningProcess()
	if err != nil || p == nil {
		state := service.getCachedProcessState()
		if state.Status == STOPPED || state.Status == EXITED {
			service.WriteServiceProcess(ServiceProcess{
				Status: state.Status,
				Pid:    NO_PID,
			})
		} else {
			service.WriteServiceProcess(ServiceProcess{
				Status: STOPPED,
				Pid:    NO_PID,
			})
		}
	} else {
		// process is already running / stopping
	}

	return service
}

func (s *Service) GetServiceProcessFilename() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("service-%s.lid", s.Name))
}

func (s *Service) getCachedProcessState() ServiceProcess {
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
	s.Logger.Printf("Writing cache: %v\n", sp)
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

	if s.EnvFile != "" {
		envPath, _ := getRelativePath(filepath.Join(s.Cwd, s.EnvFile))
		userDefinedEnv, err := ReadDotEnvFile(envPath)

		if err != nil {
			return nil, err
		}

		cmd.Env = append(os.Environ(), userDefinedEnv...)
	}

	s.Logger.Println("Starting")

	// cmd.Stdout = s.Stdout
	// cmd.Stderr = s.Stderr

	// cmd.Stdout = stdoutWriter
	// cmd.Stderr = stdoutWriter
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

			fmt.Println("threaded checking...")
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

	reader, err := cmd.StdoutPipe()

	if err != nil {
		s.Logger.Printf("%v\n", err)
		return err
	}

	// var buffer bytes.Buffer
	// teeReader := io.TeeReader(reader, &buffer)
	// teeReader := io.TeeReader(reader, &buffer)


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

	go io.Copy(s.Stdout, reader)

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
		s.Logger.Println("Exited")
		s.WriteServiceProcess(ServiceProcess{
			Status: EXITED,
			Pid:    NO_PID,
		})

		if s.OnExit != nil {
			s.OnExit(err.(*exec.ExitError), s)
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

	process, err := s.GetRunningProcess()
	if err != nil {
		return fmt.Errorf("service already down")
	}
	running, err := process.IsRunning()

	if err == nil && !running {
		return fmt.Errorf("service already down")
	}

	s.Logger.Println("Stopping service")

	s.WriteServiceProcess(ServiceProcess{
		Status: STOPPING,
		Pid:    int32(process.Pid),
	})

	terminated := make(chan bool)
	go func() {
		err = process.Terminate()

		if err != nil {
			terminated <- false
		} else {
			terminated <- true
		}
	}()

	select {
	case <-terminated:
		return nil
	case <-time.After(s.GracefulShutdownTimeout):
		return fmt.Errorf("failed to terminate service: timeout")
	}
}

func (s *Service) GetCachedStatus() ServiceStatus {
	ps := s.getCachedProcessState()
	return ps.Status
}
