package main

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

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

	const START_MS = 500
	const LOOP_MS = 250
	// Background worker that processes jobs
	manager.Register("worker", lid.ServiceConfig{
		Cwd:                   "../../mock_services",
		Command:               []string{ "bash", "slow_start.sh", strconv.Itoa(START_MS), strconv.Itoa(LOOP_MS) },
		ReadinessCheckTimeout: 1 * time.Second,
		StdoutReadinessCheck: func(line string) bool {
			return strings.Contains(line, "Started")
		},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("Worker exited unexpectedly??")
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
