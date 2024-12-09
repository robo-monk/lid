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
	Lid		*Lid
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

func (s *Service) Start() error {

	if s.GetStatus() == RUNNING {
		return fmt.Errorf("Service '%s' is already running\n", s.Name)
	}
	// process, err := s.GetProcess()

	// if err == nil {
	// 	if running, _ := process.IsRunning(); running {
	// 		// log.Fatalf("Service '%s' is already running\n", s.Name)
	// 	}

	// 	os.Remove(s.GetServiceProcessFilename())
	// }


	fmt.Printf("Starting service %s\n", s.Name);
	fmt.Printf("Current status is %d\n", s.GetStatus());

	cmd := exec.Command(s.Command[0], s.Command[1:]...)

	if s.Cwd != "" {
		cmd.Dir, _ = filepath.Abs(s.Cwd)
	}

	if s.EnvFile != "" {
		cmd.Env = ReadDotEnvFile(filepath.Join(s.Cwd, s.EnvFile))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr


	// Start the command in the background
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("Failed to start command: %v", err)
	}

	// Get the PID of the background process
	s.Lid.Logger.Printf("Started '%s' with PID: %d", s.Name, cmd.Process.Pid)

	s.WriteServiceProcess(ServiceProcess {
		Status: RUNNING,
		Pid: 	int32(cmd.Process.Pid),
	})

	err = cmd.Wait()

	s.Lid.Logger.Printf("'%s' exited\n", s.Name)

	// s.GetStatus()
	// check if we got stopped with ./lid stop

	if s.getCachedProcessState().Status != STOPPED {
		s.WriteServiceProcess(ServiceProcess {
			Status: EXITED,
			Pid:	NO_PID,
		})

		if s.OnExit != nil {
			s.OnExit(err.(*exec.ExitError), s)
		}
	};

	s.Lid.Logger.Printf("Command exited with error: %v", err)
	return nil
}

func (s *Service) Stop() error {
	process, err := s.GetProcess()
	if err != nil {
		log.Printf("Service '%s' is already down\n", s.Name)
		return err
	}

	log.Printf("Stopping service '%s'\n", s.Name)

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
