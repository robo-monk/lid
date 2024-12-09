package lid

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type Lid struct {
	services map[string]*Service
	logger *log.Logger
}

func New() *Lid {
	logFile, err := os.OpenFile("lid.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return &Lid {
		logger: log.New(logFile, "", log.Ldate|log.Ltime),
		services: make(map[string]*Service),
	}
}

func (lid *Lid) Register(serviceName string, s *Service) {
	logFile, _ := os.OpenFile("lid.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	s.Name = serviceName;
	s.Logger = log.New(logFile, fmt.Sprintf("[%s] ", s.Name), log.Ldate|log.Ltime)
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

func Contains[T comparable](s []T, e T) bool {
    for _, v := range s {
        if v == e {
            return true
        }
    }
    return false
}

func (lid *Lid) Start(services []string) {
	for _, service := range lid.services {
		if len(services) > 0 {
			if !Contains(services, service.Name) {
				continue
			}
		}

		status := service.GetStatus()
		if status == RUNNING {
			log.Printf("%s: Running with PID %d\n", service.Name, service.GetPid())
		} else {
			log.Printf("%s: Starting \n", service.Name)
			_, err := service.PrepareCommand()
			if err != nil {
				log.Printf("%v\n", err)
			} else {
				lid.Fork("--start-service", service.Name)
			}
		}
	}
}

func (lid *Lid) Stop(services []string) {
	for _, service := range lid.services {
		if len(services) > 0 {
			if !Contains(services, service.Name) {
				continue
			}
		}

		err := service.Stop()
		if err != nil {
			log.Printf("%s: %v\n", service.Name, err)
		} else {
			log.Printf("%s: Stopped\n", service.Name)
		}
	}
}

func (lid *Lid) List() {
	for _, service := range lid.services {
		proc, err := service.GetProcess()

		if err != nil {
			log.Printf("%s	| STOPPED\n", service.Name)
			continue
		}

		createTime, _ := proc.CreateTime()
		upTime := time.Now().UnixMilli() - createTime
		cpu, _ := proc.CPUPercent()

		log.Printf("%s	| RUNNING (%d secs) [%f%%]\n", service.Name, (upTime / 1000), cpu)
	}
}

const invalidUsage = "invalid usage"

func (lid *Lid) Run() {
	if len(os.Args) < 2 {
		panic(invalidUsage)
	}

	switch os.Args[1] {
	case "start":
		lid.Start(os.Args[2:])
	case "stop":
		lid.Stop(os.Args[2:])
	case "restart":
		lid.Stop(os.Args[2:])
		lid.Start(os.Args[2:])
	case "ls":
		fallthrough
	case "list":
		lid.List()
	case "--start-service":
		serviceName := os.Args[2]
		lid.logger.Printf("Starting %s\n", serviceName)
		err := lid.services[serviceName].Start()
		if err != nil {
			lid.logger.Printf("Could not start %s: %v\n", serviceName, err)
		}
	default:
		// panic("Invalid usage")
		panic(invalidUsage)
	}
}
