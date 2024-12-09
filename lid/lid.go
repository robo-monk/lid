package lid

import (
	"log"
	"os"
	"os/exec"
	"time"
)

type Lid struct {
	// services []Service
	services map[string]*Service
	Logger *log.Logger
}

func New() *Lid {
	logFile, err := os.OpenFile("lid.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return &Lid {
		Logger: log.New(logFile, "", log.Ldate|log.Ltime),
		services: make(map[string]*Service),
	}
}

func (lid *Lid) Register(serviceName string, s *Service) {
	s.Name = serviceName;
	s.Lid = lid
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

		proc, _ := service.GetProcess()
		if proc != nil {
			log.Printf("Service '%s' is already running with PID %d\n", service.Name, proc.Pid)
			continue
		}

		log.Printf("Starting '%s' \n", service.Name)
		lid.Fork("--start-service", service.Name)
	}

}

func (lid *Lid) Stop() {
	for _, service := range lid.services {
		err := service.Stop()
		if err == nil {
			lid.Logger.Printf("Stop %s\n", service.Name)
		}
	}
}

func (lid *Lid) List() {
	for _, service := range lid.services {
		proc, _ := service.GetProcess()

		if proc == nil {
			log.Printf("%s	| STOPPED\n", service.Name)
			continue
		}

		createTime, _ := proc.CreateTime()
		upTime := time.Now().UnixMilli() - createTime
		cpu, _ := proc.CPUPercent()

		log.Printf("%s	| RUNNING (%d mins) [%f%%]\n", service.Name, (upTime / 1000 / 60), cpu)
	}
}

const invalidUsage = "invalid usage"

func (lid *Lid) Run() {
	if len(os.Args) < 2 {
		panic(invalidUsage)
	}

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
	case "--start-service":
		serviceName := os.Args[2]
		lid.services[serviceName].Start()
	default:
		// panic("Invalid usage")
		panic(invalidUsage)
	}
}
