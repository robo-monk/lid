package lid

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/aquasecurity/table"
)

type Lid struct {
	services map[string]*Service
	logger   *log.Logger
}

func New() *Lid {
	logFile, err := os.OpenFile("lid.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return &Lid{
		logger:   log.New(logFile, "", log.Ldate|log.Ltime),
		services: make(map[string]*Service),
	}
}

func (lid *Lid) Register(serviceName string, s *Service) {
	if lid.services[serviceName] != nil {
		log.Fatalf("Cannot register '%s' service twice.\n", serviceName)
	}

	logFile, _ := os.OpenFile("lid.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	s.Name = serviceName
	s.Logger = log.New(io.MultiWriter(os.Stdout, logFile), fmt.Sprintf("[%s] ", s.Name), log.Ldate|log.Ltime)
	lid.services[serviceName] = s
}

func (lid *Lid) Fork(args ...string) {
	// exe
	executablePath, _ := os.Executable()
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

		proc, err := service.GetRunningProcess()
		if err == nil {
			log.Printf("%s: Running with PID %d\n", service.Name, proc.Pid)
		} else {
			log.Printf("%s: Starting \n", service.Name)
			_, err := service.PrepareCommand()
			if err != nil {
				log.Printf("%s: %v\n", service.Name, err)
			} else {
				lid.Fork("spawn", service.Name)
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
	t := table.New(os.Stdout)

	t.SetHeaders("Name", "Status", "Uptime", "PID", "CPU", "Memory")

	// t.AddRow("1", "Apple", "14")
	//
	keys := make([]string, 0, len(lid.services))
	for key := range lid.services {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// for _, service := range lid.services {
	for _, serviceName := range keys {
		service := lid.services[serviceName]
		proc, err := service.GetRunningProcess()

		if err != nil {
			t.AddRow(service.Name, "\033[31mStopped\033[0m", "0", "-", "-")
			continue
		}

		createTime, _ := proc.CreateTime()
		upTime := time.Now().UnixMilli() - createTime
		cpu, _ := proc.CPUPercent()
		mem, _ := proc.MemoryInfo()
		pid := proc.Pid

		t.AddRow(
			service.Name,
			"\033[32mRunning\033[0m",
			fmt.Sprintf("%ds", upTime/1000),
			fmt.Sprintf("%d", pid),
			fmt.Sprintf("%f%%", cpu),
			fmt.Sprintf("%dMB", mem.RSS/1000000),
		)
	}

	t.Render()
}

func (lid *Lid) Logs(services []string) {
	var wg sync.WaitGroup

	for _, service := range lid.services {
		if len(services) > 0 {
			if !Contains(services, service.Name) {
				continue
			}
		}

		logFile, err := os.Open(service.GetServiceLogFilename())
		if err != nil {
			log.Printf("Could not open log file for service %s: %v", service.Name, err)
			continue
		}
		logFile.Close()

		wg.Add(1)
		go func(serviceName string, logFilename string) {
			defer wg.Done()
			log.Printf("Tailing file %s\n", logFilename)
			cmd := exec.Command("tail", "-n", "20", "-f", logFilename)

			// Prefix each line with service name
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					fmt.Printf("[%s] %s\n", serviceName, scanner.Text())
				}
			}()

			go func() {
				scanner := bufio.NewScanner(stderr)
				for scanner.Scan() {
					fmt.Printf("[%s][ERR] %s\n", serviceName, scanner.Text())
				}
			}()

			cmd.Run()
		}(service.Name, service.GetServiceLogFilename())
	}

	wg.Wait()
}

func (lid *Lid) GetUsage() string {
	usage := `lid CLI

Usage:
  lid [command]

Available commands:
	list			Lists the status of all services
	start			Starts all registered services
	start <service>		Starts a specific service
	stop			Stops all running services
	stop <service>		Stops a specific service
	restart 		Restarts all services
	restart <service>	Restarts a specific service
	logs				Tails the logs of all services
	logs <service>		Tails the logs of a specific service
	spawn <service>		Spawns and attaches to the service. Meant for debugging

Available services:
`

	if len(lid.services) == 0 {
		usage += "  (No services registered)\n"
	} else {
		for serviceName := range lid.services {
			usage += fmt.Sprintf("  - %s\n", serviceName)
		}
	}

	return usage
}

func (lid *Lid) Run() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal(lid.GetUsage())
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
	case "logs":
		lid.Logs(os.Args[2:])
	case "spawn":
		serviceName := os.Args[2]
		log.Printf("Spawning '%s'\n", serviceName)
		lid.logger.Printf("Starting %s\n", serviceName)
		err := lid.services[serviceName].Start()
		if err != nil {
			lid.logger.Printf("Could not start %s: %v\n", serviceName, err)
		}
	default:
		log.Fatal(lid.GetUsage())
	}
}
