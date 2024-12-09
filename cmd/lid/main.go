package main

import (
	"os/exec"
	"github.com/robo-monk/lid/lid"
)

func main() {

    manager := lid.New()

    manager.Register("pocketbase", &lid.Service{
        Cwd:     "../pocketbase",
        Command: []string{"./pocketbase", "serve"},
        EnvFile: ".env",
        OnExit: func(e *exec.ExitError, service *lid.Service) {
            // if service.GetStatus() != lid.STOPPED {
           	manager.Logger.Println("POCKETBASE FAILED")
           	service.Start()
            // }
        },
    })

    manager.Run()
}
