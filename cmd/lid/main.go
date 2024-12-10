package main

import (
	"os/exec"
	"github.com/robo-monk/lid/lid"
)

func main() {

    manager := lid.New()

    manager.Register("pocketbase", &lid.Service{
        Cwd:     "../../../../convex/convex/pocketbase",
        Command: []string{"./convex-pb", "serve"},
        EnvFile: ".env",
        OnExit: func(e *exec.ExitError, service *lid.Service) {
           	service.Logger.Println("POCKETBASE FAILED")
           	service.Start()
        },
    })

    manager.Register("test-recur", &lid.Service{
        Command: []string{"bash", "-c", "sleep 2; exit 1"},
        OnExit: func(e *exec.ExitError, service *lid.Service) {
           	service.Logger.Println("FAILED")
           	service.Start()
        },
    })


    manager.Register("b", &lid.Service {
      	Cwd: "../../../../convex/convex/convex-insights/scrape-server",
       	EnvFile: ".env",
       	Command: []string { "bun", "run", "./src/index.ts" },
       	OnExit: func(e *exec.ExitError, service *lid.Service) {
          		service.Logger.Println("scraper failed...")
        },
    })


    manager.Run()
}
