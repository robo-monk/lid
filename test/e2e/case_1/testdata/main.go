package main

import (
	"os/exec"

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
	manager.Register("worker", &lid.Service{
		Command: []string{"bash", "../../mock_services/long_running.sh"},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("Worker exited unexpectedly, restarting...")
			service.Start()
		},
	})

	// Crash-prone service to test recovery
	manager.Register("unstable-service", &lid.Service{
		Command: []string{"bash", "../../mock_services/crash_service.sh"},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("Unstable service crashed, restarting with backoff...")
			service.Start()
		},
	})

	manager.Run()
}
