package lid

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aquasecurity/table"
)

type Lid struct {
	services     map[string]*Service
	logsFilename string
	logger       *log.Logger
}

type LidOptions struct {
	LogsFilename string
}

func NewWithOptions(options LidOptions) (*Lid, error) {
	logFile, err := os.OpenFile(options.LogsFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Lid{
		logsFilename: options.LogsFilename,
		logger:       log.New(logFile, "", log.Ldate|log.Ltime),
		services:     make(map[string]*Service),
	}, nil
}

func New() *Lid {
	logsFilename, err := getRelativePath("lid.log")
	if err != nil {
		log.Fatalln(err)
	}

	lid, err := NewWithOptions(LidOptions{
		LogsFilename: logsFilename,
	})
	if err != nil {
		log.Fatalln(err)
	}
	return lid
}

func (lid *Lid) Register(serviceName string, s ServiceConfig) {
	if _, ok := lid.services[serviceName]; ok {
		log.Fatalf("Cannot register '%s' service twice.\n", serviceName)
	}

	logFile, _ := os.OpenFile(lid.logsFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if s.Logger == nil {
		logger := log.New(io.MultiWriter(os.Stdout, logFile), fmt.Sprintf("[%s] ", serviceName), log.Ldate|log.Ltime)

		if s.Stdout == nil {
			s.Stdout = logger.Writer()
		}

		if s.Stderr == nil {
			s.Stderr = logger.Writer()
		}

		s.Logger = logger
	}

	lid.services[serviceName] = NewService(serviceName, s)
}

func (lid *Lid) ForkSpawn(serviceName string) {
	service, ok := lid.services[serviceName]
	if !ok {
		lid.logger.Printf("Service '%s' not found\n", serviceName)
		return
	}

	// exe
	executablePath, _ := os.Executable()
	cmd := exec.Command(executablePath, "spawn", serviceName)

	// temp process indpendent file
	tempFile, err := os.CreateTemp("", "lid-spawn-")

	if err != nil {
		service.Logger.Printf("Failed to create temp file: %v\n", err)
		return
	}

	// Remove the temp file when done
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	cmd.Stdout = tempFile
	cmd.Stderr = tempFile

	// Start command in background
	if err := cmd.Start(); err != nil {
		service.Logger.Printf("Failed to start service: %v\n", err)
		return
	}

	readyChan := make(chan bool)
	start := time.Now()
	timeout := time.After(service.ReadinessCheckTimeout)

	go tailFile(tempFile.Name(), func(line string) bool {
		fmt.Print("\t", line)
		if strings.Contains(line, READINESS_CHECK_PASSED_MESSAGE) || strings.Contains(line, NO_READINESS_CHECK_MESSAGE) {
			readyChan <- true
			return true
		}
		return false
	})

	// wait for "Readiness check passed" with timeout
	select {
	case <-readyChan:
		service.Logger.Printf("Started successfully in %s\n", time.Since(start))
	case <-timeout:
		service.Logger.Printf("Warning: Service is taking longer than %.2f second(s) to start. Consider configuring the service's 'ReadinessCheckTimeout'.", service.ReadinessCheckTimeout.Seconds())
	}

	// Detach the process
	cmd.Process.Release()
	service.Logger.Printf("Detached process\n")
}

func (lid *Lid) Start(services []string) {
	var wg sync.WaitGroup

	for _, service := range lid.services {
		if len(services) > 0 {
			if !contains(services, service.Name) {
				continue
			}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			proc, err := service.GetRunningProcess()
			if err == nil {
				service.Logger.Printf("Running with PID %d\n", proc.Pid)
			} else {
				lid.ForkSpawn(service.Name)
			}
		}()
	}

	wg.Wait()
}

func (lid *Lid) Stop(services []string) {
	lid.logger.Println("Stopping services")
	var wg sync.WaitGroup

	for _, service := range lid.services {
		if len(services) > 0 {
			if !contains(services, service.Name) {
				continue
			}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := service.Stop()
			if err != nil {
				service.Logger.Printf("%s: %v\n", service.Name, err)
			} else {
				service.Logger.Printf("%s: Stopped\n", service.Name)
			}
		}()
	}

	wg.Wait()
	lid.logger.Println("Services stopped")
}

func (lid *Lid) List() {
	t := table.New(os.Stdout)

	t.SetHeaders("Name", "Status", "Uptime", "PID", "CPU", "Memory")

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

		status := service.GetCachedStatus()
		statusStr := ""
		if status == STARTING {
			statusStr = "\033[33mStarting\033[0m"
		} else if status == RUNNING {
			statusStr = "\033[32mRunning\033[0m"
		} else if status == STOPPED {
			statusStr = "\033[31mStopped\033[0m"
		}

		t.AddRow(
			service.Name,
			statusStr,
			fmt.Sprintf("%ds", upTime/1000),
			fmt.Sprintf("%d", pid),
			fmt.Sprintf("%f%%", cpu),
			fmt.Sprintf("%dMB", mem.RSS/1000000),
		)
	}

	t.Render()
}

func (lid *Lid) Logs(services []string) {
	log.Printf("Tailing file %s\n", lid.logsFilename)
	cmd := exec.Command("tail", "-n", "20", "-f", lid.logsFilename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
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
		lid.logger.Printf("Starting %s\n", serviceName)
		err := lid.services[serviceName].Start()
		if err != nil {
			lid.logger.Printf("Could not start %s: %v\n", serviceName, err)
		}
	default:
		log.Fatal(lid.GetUsage())
	}
}
