package lid

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	Status 	ServiceStatus
	Pid		int32
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
	Logger  *log.Logger
	Name 	string
	Cwd		string

	Command []string
	EnvFile	string

	// OnStop 	func(e *exec.ExitError, self *Service)
	OnExit 	func(e *exec.ExitError, self *Service)
}

func (s *Service) GetServiceProcessFilename() string {
	return fmt.Sprintf("/tmp/service-%s.lid", s.Name)
}

func (s *Service) getCachedProcessState() ServiceProcess {
	sp, error := ReadServiceProcess(s.GetServiceProcessFilename())
	if error != nil {
		return ServiceProcess {
			Pid: NO_PID,
			Status: STOPPED,
		}
	}
	return sp
}

func (s *Service) WriteServiceProcess(sp ServiceProcess) error {
	return sp.WriteToFile(s.GetServiceProcessFilename())
}

func (s *Service) GetProcess() (*process.Process, error) {
	proc := s.getCachedProcessState()
	return  process.NewProcess(int32(proc.Pid))
}

func (s *Service) GetPid() int32 {
	return s.getCachedProcessState().Pid
}

func (s *Service) PrepareCommand() (*exec.Cmd, error) {
	if s.GetStatus() == RUNNING {
		return nil, fmt.Errorf("Service '%s' is already running\n", s.Name)
	}

	cmd := exec.Command(s.Command[0], s.Command[1:]...)

	if s.Cwd != "" {
		cmd.Dir, _ = filepath.Abs(s.Cwd)
	}

	if s.EnvFile != "" {
		var ferr error
		cmd.Env, ferr = ReadDotEnvFile(filepath.Join(s.Cwd, s.EnvFile))
		if ferr != nil{
			return nil, ferr
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func (s *Service) Start() error {

	cmd, err := s.PrepareCommand()
	if err != nil {
		s.Logger.Printf("%v\n", err);
	}

	s.Logger.Printf("START %v\n", cmd);

	err = cmd.Start()
	if err != nil {
		ferr := fmt.Errorf("Failed to start command: %v", err)
		s.Logger.Printf("%v\n", ferr);
		return ferr
	}

	// Get the PID of the background process
	s.Logger.Printf("Started with PID: %d", cmd.Process.Pid)

	s.WriteServiceProcess(ServiceProcess {
		Status: RUNNING,
		Pid: 	int32(cmd.Process.Pid),
	})

	err = cmd.Wait()

	if err != nil {
		s.Logger.Printf("%v\n", err)
	}

	if s.getCachedProcessState().Status != STOPPED {
		s.Logger.Println("Exited")
		s.WriteServiceProcess(ServiceProcess {
			Status: EXITED,
			Pid:	NO_PID,
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
	process, err := s.GetProcess()
	if err != nil {
		return fmt.Errorf("Service already down")
	}

	s.Logger.Println("Stopping service")

	s.WriteServiceProcess(ServiceProcess {
		Status: STOPPED,
		Pid:    NO_PID,
	})

	recursiveKill(process)
	return nil
}

func (s *Service) GetStatus() ServiceStatus {
	ps := s.getCachedProcessState()
	exists, err := process.PidExists(ps.Pid)
	if err != nil && !exists && ps.Status == RUNNING {
		return STOPPED
	}
	return ps.Status
}


func recursiveKill(p *process.Process) {
	children, _ := p.Children()
	for _, child := range children {
		recursiveKill(child)
	}
	p.Kill()
}
