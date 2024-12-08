package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

type ExitEvent struct {
	code int
}
type Service struct {

	name 	string
	cwd		string
	cmd		string

	onExit 	func(e ExitEvent)
}


type Lid struct {
	services []Service
	logger *log.Logger
}

func New() *Lid {
	logFile, err := os.OpenFile("lid.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return &Lid {
		logger: log.New(logFile, "LOG: ", log.Ldate|log.Ltime|log.Lshortfile),
		services: []Service {},
	}
}

func (lid *Lid) Register(serviceName string, s Service) {
	s.name = serviceName;
	lid.services = append(lid.services, s)
}

func wrap(s Service) string {
	lidExec, _ := os.Executable()
	cmd := ""
	cmd += fmt.Sprintf("%s --event start %s;", lidExec, s.name)
	cmd += fmt.Sprintf("(%s);", s.cmd)
	cmd += "exit_code=$?;"
	cmd += fmt.Sprintf("%s --event exited %s $exit_code", lidExec, s.name)
	fmt.Println(cmd)
	return cmd
}

func (s Service) getPidFilename() string {
	return fmt.Sprintf("/tmp/lid-%s.pid", s.name)
}


var (
	ErrProcessNotFound = fmt.Errorf("Process Not Found")
	ErrProcessCorrupt = fmt.Errorf("Process Not Found")
	// ErrProcessNotFound = fmt.Errorf("Process Not Found")
)

func (s Service) getProcess() (*process.Process, error) {
	pidContent, fileErr := os.ReadFile(s.getPidFilename())

	if fileErr != nil {
		if os.IsNotExist(fileErr) {
			return nil, ErrProcessNotFound
		} else {
			// log.Printf("%s	| CORRUPT  (%s)\n", service.name, fileErr.Error())
			return nil, ErrProcessCorrupt
		}
	} else {
		pid, err := strconv.Atoi(string(pidContent))

		if err != nil {
			return nil, ErrProcessCorrupt
		}

		p, err  := process.NewProcess(int32(pid))

		if err != nil {
			os.Remove(s.getPidFilename());
			return nil, ErrProcessNotFound
		}

		return p, nil
	}
}

func recursiveKill(p *process.Process) {
	children, _ := p.Children()
	for _, child := range children {
		recursiveKill(child)
	}
	p.Kill()
}


const invalidUsage = ("Invalid usage. Usage lid start | status")
func main() {
	// executablePath, _ := os.Executable();

	// fmt.Println("os.Executable", executablePath);
	// fmt.Println(len(os.Args), os.Args)

	if len(os.Args) < 2 {
		panic(invalidUsage)
	}

	lid := New()

	lid.Register("pocketbase", Service {
		cmd: 	`cd ../../convex/convex/pocketbase && dotenv -- ./convex-pb serve`,
		onExit: func (e ExitEvent) {
			fmt.Printf("Process exited\n")
		},
	});


	lid.Register("test", Service {
		cmd: 	`sleep 4; exit 42`,
		onExit: func (e ExitEvent) {
			fmt.Printf("Process exited\n")
		},
	});

	switch os.Args[1] {
	case "start":
		for _, service := range lid.services {

			proc, _ := service.getProcess()
			if proc != nil {
				log.Printf("service '%s' is already running with PID %d \n", service.name, proc.Pid)
				continue
			}

			fmt.Printf("Starting service %s\n", service.name);

			fmt.Println("Starting command..")
			cmd := exec.Command("bash", "-c", wrap(service))
			// cmd.Stdout = os.Stdout
			// cmd.Stderr = os.Stderr

			// Start the command in the background
			err := cmd.Start()
			if err != nil {
				log.Fatalf("Failed to start command: %v", err)
			}

			// Get the PID of the background process
			log.Printf("Started process with PID: %d", cmd.Process.Pid)
			pid := cmd.Process.Pid;
			filename := service.getPidFilename()
			os.WriteFile(filename, []byte(strconv.Itoa(pid)), 0666)
			log.Printf("Registering PID file '%s'\n", filename)
		}
	case "stop":
		for _, service := range lid.services {
			process, err := service.getProcess()
			if err != nil {
				log.Printf("Service '%s' is already down\n", service.name)
				continue;
			}

			log.Printf("Stopping '%s \n", service.name)
			recursiveKill(process)
			lid.logger.Printf("Stop %s\n", service.name)
		}
	case "list":
		for _, service := range lid.services {
			proc, _ := service.getProcess()

			if proc == nil {
				log.Printf("%s	| STOPPED\n", service.name)
				continue
			}

			createTime, _ := proc.CreateTime()
			upTime := time.Now().UnixMilli() - createTime
			cpu, _ := proc.CPUPercent()

			log.Printf("%s	| RUNNING (%d mins) [%f%%]\n", service.name, (upTime / 1000 / 60), cpu)
		}
	case "--event":
		lid.logger.Printf("EVENT: %s\n", strings.Join(os.Args, ", "))
	default:
		panic(invalidUsage)
	}

}
