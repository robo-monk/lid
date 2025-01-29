package main

import (
	"os/exec"
	"strings"
	"syscall"

	"github.com/robo-monk/lid/lid"
)

func main() {

	manager := lid.New()
	manager.Register("pocketbase", lid.ServiceConfig{
		Cwd:     "../../../convex/convex/pocketbase",
		Command: []string{"./convex-pb", "serve"},
		EnvFile: ".env",
		StdoutReadinessCheck: func(line string) bool {
			return strings.Contains(line, "Server started at")
		},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("POCKETBASE FAILED")
			service.Start()
		},
	})

	// simulate a container with slow startup and a slow shutdown
	manager.Register("container", lid.ServiceConfig{
		// trap SIGINT and exit gracefully after 3 seconds
		// print hello after 3 seconds
		Command: []string{"bash", "-c", "trap 'sleep 3; exit 0' SIGINT; sleep 3; echo 'hello'; while true; do sleep 1; done"},
		StdoutReadinessCheck: func(line string) bool {
			return strings.Contains(line, "hello")
		},
		OnAfterStart: func(service *lid.Service) {
			service.Logger.Println("Container started")
		},
		ExitSignal: syscall.SIGINT,
	})

	manager.Register("test-recur", lid.ServiceConfig{
		Command: []string{"bash", "-c", "sleep 4; exit 1"},
		OnAfterStart: func(service *lid.Service) {
			service.Logger.Println("hello")
		},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("> Failed")
			service.Start()
		},
	})

	manager.Register("b", lid.ServiceConfig{
		Cwd:     "../../../convex/convex/convex-insights/scrape-server",
		EnvFile: ".env",
		Command: []string{"bun", "run", "./src/index.ts"},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("scraper failed...")
		},
	})

	manager.Run()
}
