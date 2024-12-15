package main

import (
	"os/exec"
	"strings"

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
