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

	Stdout io.Writer
	Stderr io.Writer
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

	if s.Stderr != nil {
		cmd.Stderr = s.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	if s.Stdout != nil {
		cmd.Stdout = s.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	return cmd, nil
}

func (s *Service) Start() error {

	cmd, err := s.PrepareCommand()
	if err != nil {
		s.Logger.Printf("%v\n", err)
		return err
	}

	s.Logger.Printf("Command: %v\n", cmd)

	if s.OnBeforeStart != nil {
		err := s.OnBeforeStart(s)
		if err != nil {
			s.Logger.Printf("rejected start: %v\n", err)
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

	s.WriteServiceProcess(ServiceProcess {
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

	recursiveKill(process)
	return nil
}

func (s *Service) GetCachedStatus() ServiceStatus {
	ps := s.getCachedProcessState()
	return ps.Status
}

func recursiveKill(p *process.Process) {
	children, _ := p.Children()
	for _, child := range children {
		recursiveKill(child)
	}
	p.Kill()
}
