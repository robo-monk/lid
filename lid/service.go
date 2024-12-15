package lid

import (
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

const (
	STOPPED ServiceStatus = iota
	EXITED
	STARTING
	RUNNING
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

	OnBeforeStart func(self *Service) error
	OnAfterStart  func(self *Service)
	OnExit        func(e *exec.ExitError, self *Service)
}

type ServiceConfig struct {
	Cwd           string
	Command       []string
	EnvFile       string
	OnBeforeStart func(self *Service) error
	OnAfterStart  func(self *Service)
	OnExit        func(e *exec.ExitError, self *Service)
}

func (s *Service) GetServiceProcessFilename() string {
	return fmt.Sprintf("/tmp/service-%s.lid", s.Name)
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
	cmd.Stdout = io.MultiWriter(os.Stdout, s.Logger.Writer())
	cmd.Stderr = io.MultiWriter(os.Stderr, s.Logger.Writer())

	// stdout, _ := cmd.StdoutPipe()
	// scanner := bufio.NewScanner(stdout)

	// targetMessage := "bing"
	// for scanner.Scan() {
	// 	line := scanner.Text()
	// 	if strings.Contains(line, targetMessage) {
	// 		break
	// 	}
	// }

	return cmd, nil
}

func (s *Service) Start() error {

	cmd, err := s.PrepareCommand()
	if err != nil {
		s.Logger.Printf("%v\n", err)
		return err
	}

	s.Logger.Printf("Running Command: %v\n", cmd)

	if s.OnBeforeStart != nil {
		err := s.OnBeforeStart(s)
		if err != nil {
			s.Logger.Printf("Rejected start: %v\n", err)
			return err
		}
	}

	err = cmd.Start()

	if err != nil {
		err := fmt.Errorf("failed to start command: %v", err)
		s.Logger.Printf("%v\n", err)
		return err
	}

	// Get the PID of the background process
	s.Logger.Printf("Started with PID: %d", cmd.Process.Pid)

	s.WriteServiceProcess(ServiceProcess{
		Status: RUNNING,
		Pid:    int32(cmd.Process.Pid),
	})

	if s.OnAfterStart != nil {
		s.OnAfterStart(s)
	}

	err = cmd.Wait()

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

	return nil
}

func (s *Service) Stop() error {
	process, err := s.GetRunningProcess()
	if err != nil {
		return fmt.Errorf("Service already down")
	}

	s.Logger.Println("Stopping service")

	s.WriteServiceProcess(ServiceProcess{
		Status: STOPPED,
		Pid:    NO_PID,
	})

	errors := recursiveTerminate(process)

	if len(errors) > 0 {
		return fmt.Errorf("failed to terminate service: %v", errors)
	}

	return nil
}

func (s *Service) GetCachedStatus() ServiceStatus {
	ps := s.getCachedProcessState()
	return ps.Status
}

func recursiveTerminate(p *process.Process) []error {
	errors := []error{}

	children, _ := p.Children()
	for _, child := range children {
		err := recursiveTerminate(child)
		if err != nil {
			errors = append(errors, err...)
		}
	}

	err := p.Terminate()

	if err != nil {
		errors = append(errors, err)
	}

	now := time.Now()
	deadline := now.Add(5 * time.Second)

	for {
		running, err := p.IsRunning()
		if !running || err != nil {
			break
		}
		if time.Now().After(deadline) {
			errors = append(errors, fmt.Errorf("killed process by force -- timeout"))
			recursiveKill(p)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return errors
}

func recursiveKill(p *process.Process) {
	children, _ := p.Children()
	for _, child := range children {
		recursiveKill(child)
	}
	p.Kill()
}
