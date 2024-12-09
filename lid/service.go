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

	OnFail 	func(e *exec.ExitError, self *Service)
	OnExit 	func(e *exec.ExitError, self *Service)
}



func (s *Service) GetServiceProcessFilename() string {
	return fmt.Sprintf("/tmp/service-%s.lid", s.Name)
}

func (s *Service) syncProcessState() ServiceProcess {
	sp, error := ReadServiceProcess(s.GetServiceProcessFilename())
	if error != nil {
		return ServiceProcess {
			Pid: NO_PID,
			Status: STOPPED,
		}
	}

	exists, error := process.PidExists(int32(sp.Pid))

	// sync file
	if error != nil || !exists {
		sp.Pid = NO_PID
		if (sp.Status != STOPPED) && (sp.Status != EXITED) {
			sp.Status = STOPPED
		}
		s.WriteServiceProcess(sp)
	}

	return sp
}

func (s *Service) WriteServiceProcess(sp ServiceProcess) error {
	return sp.WriteToFile(s.GetServiceProcessFilename())
}

func (s *Service) GetProcess() (*process.Process, error) {
	proc := s.syncProcessState()
	return  process.NewProcess(int32(proc.Pid))
}

func (s *Service) GetPid() int32 {
	return s.syncProcessState().Pid
}

func (s *Service) Start() {

	process, err := s.GetProcess()

	if err == nil {
		if running, _ := process.IsRunning(); running {
			log.Fatalf("Service '%s' is already running\n", s.Name)
		}

		os.Remove(s.GetServiceProcessFilename())
	}


	fmt.Printf("Starting service %s\n", s.Name);

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
	err = cmd.Start()
	if err != nil {
		s.Lid.Logger.Fatalf("Failed to start command: %v", err)
	}

	// Get the PID of the background process
	s.Lid.Logger.Printf("Started '%s' with PID: %d", s.Name, cmd.Process.Pid)

	sp := ServiceProcess {
		Status: RUNNING,
		Pid:    int32(cmd.Process.Pid),
	}

	s.WriteServiceProcess(sp)
	err = cmd.Wait()

	s.Lid.Logger.Printf("'%s' exited\n", s.Name)

	// check if we got stopped with ./lid stop
	if sp.Status != STOPPED {
		sp.Status = EXITED
		sp.Pid = NO_PID
		s.WriteServiceProcess(sp)
	}

	sp = s.syncProcessState()

	s.Lid.Logger.Printf("Command exited with error: %v", err)

	if s.OnExit != nil {
		s.OnExit(err.(*exec.ExitError), s)
	}

	if sp.Status != STOPPED {
		if s.OnFail != nil {
			s.OnFail(err.(*exec.ExitError), s)
		}
	}
}

func (s *Service) Stop() error {
	process, err := s.GetProcess()
	if err != nil {
		log.Printf("Service '%s' is already down\n", s.Name)
		return err
	}

	log.Printf("Stopping service '%s'\n", s.Name)
	recursiveKill(process)


	proc := s.syncProcessState()
	proc.Status = STOPPED
	proc.Pid = NO_PID
	s.WriteServiceProcess(proc)
	return nil
}

func (s *Service) GetStatus() ServiceStatus {
	process := s.syncProcessState()
	return process.Status
}


func recursiveKill(p *process.Process) {
	children, _ := p.Children()
	for _, child := range children {
		recursiveKill(child)
	}
	p.Kill()
}
