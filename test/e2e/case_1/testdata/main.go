package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/robo-monk/lid/lid"
)

func main() {
	manager := lid.New()

	// Web service that should always be running
	// manager.Register("web-server", &lid.Service{
	// 	Command: []string{"./mock_services/http_server", "8080"},
	// 	OnExit: func(e *exec.ExitError, service *lid.Service) {
	// 		service.Logger.Println("Web server crashed, restarting...")
	// 		service.Start()
	// 	},
	// })

	// Background worker that processes jobs
	manager.Register("worker", lid.ServiceConfig{
		Cwd:     "../../mock_services",
		Command: []string{"bash", "long_running.sh"},
		StdoutReadinessCheck: func(line string) bool {
			// service.Logger.Println("Checking readiness:", line)
			// return strings.Contains(line, "Starting")
			fmt.Println("Checking readiness:", line)
			return strings.Contains(line, "Service is running")
		},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("Worker exited unexpectedly, restarting...")
			service.Start()
		},
	})

	// Crash-prone service to test recovery
	manager.Register("unstable-service", lid.ServiceConfig{
		Cwd:     "../../mock_services",
		Command: []string{"bash", "crash_service.sh"},
		StdoutReadinessCheck: func(line string) bool {
			return strings.Contains(line, "Starting crash-prone service...")
		},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Start()
		},
	})

	manager.Run()
}
