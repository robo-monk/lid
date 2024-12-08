package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func assert(condition bool, message string) {
	if !condition {
		log.Fatalf("Assertion error: %s\n", message)
	}
}

type Lid struct {
	// services []Service
	services map[string]*Service
	logger *log.Logger
}

func New() *Lid {
	logFile, err := os.OpenFile("lid.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return &Lid {
		logger: log.New(logFile, "LOG: ", log.Ldate|log.Ltime|log.Lshortfile),
		services: make(map[string]*Service),
	}
}

func (lid *Lid) Register(serviceName string, s *Service) {
	s.name = serviceName;
	lid.services[serviceName] = s
}


func (lid *Lid) Fork(args ...string) {
	// exe
	executablePath, _ := os.Executable();
	cmd := exec.Command(executablePath, args...)

	// fork
	cmd.Start()
	cmd.Process.Release()

}


func (lid *Lid) Start() {
	for _, service := range lid.services {

		proc, _ := service.getProcess()
		if proc != nil {
			log.Printf("Service '%s' is already running with PID %d\n", service.name, proc.Pid)
			continue
		}

		log.Printf("Starting '%s' \n", service.name)
		lid.Fork("--start-process", service.name)
	}

}

func (lid *Lid) Stop() {
	for _, service := range lid.services {
		service.Stop()
		lid.logger.Printf("Stop %s\n", service.name)
	}
}

func (lid *Lid) List() {
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
}

const invalidUsage = ("Invalid usage. Usage lid start | status")
func main() {

	if len(os.Args) < 2 {
		panic(invalidUsage)
	}

	lid := New()

	lid.Register("pocketbase", &Service {
		cwd: "../../convex/convex/pocketbase",
		command: []string { "./convex-pb", "serve"},
		envFile: ".env",
		onExit: func (ok bool, e *exec.ExitError) {
			if ok {
				lid.logger.Printf(" -- Pocketbase grafully shut down\n")
			} else {
				lid.logger.Printf(" -- Pocketbase Exited with %d; %v\n", e.ExitCode(), ok)
			}
		},
	});


	lid.Register("test", &Service {
		command: []string { "bash", "-c", "sleep 5;exit 0" },
		onExit: func (ok bool, e *exec.ExitError) {
			if ok {
				lid.logger.Printf(" -- error code ok %v\n", ok)
			} else {
				lid.logger.Printf(" -- Sleep Exited with %d; %v\n", e.ExitCode(), ok)
			}
		},
	});

	switch os.Args[1] {
	case "start":
		lid.Start()
	case "stop":
		lid.Stop()
	case "restart":
		lid.Stop()
		lid.Start()
	case "ls":
		fallthrough
	case "list":
		lid.List()
	case "--start-process":
		serviceName := os.Args[2]
		lid.services[serviceName].Start()
	case "--event":
		lid.logger.Printf("EVENT: %s\n", strings.Join(os.Args, ", "))
	default:
		panic(invalidUsage)
	}

}
