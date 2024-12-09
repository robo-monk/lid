# lid

Lid is a very simple process manager written in Go. It is a lightweight alternative to pm2 and forever.
It is inspired by the fantastic DX of [Pocketbase](https://pocketbase.io/) and follows similar patterns.

It aims to have:
- **No Background Daemon**
- **Simple Interface**
- **Configurable Behavior**
- **Zero Downtime**
- **Agnostic Process Support**

> This project is in alpha. Use with caution.

### Recommended Installation
1.  Create a `lid` directory in the root of your monorepo
2.  Create a `lid/lig.go` and register your services
4.  `go mod tidy && go build -o lid`

### CLI Usage

```bash
lid CLI

Usage:
  lid [command]

Available commands:
  start             	Starts all registered services
  start <service>   	Starts a specific service
  stop              	Stops all running services
  stop <service>    	Stops a specific service
  restart           	Restarts all services
  restart <service> 	Restarts a specific service
  list              	Lists the status of all services

Available services:
  - pocketbase
  - backend
  - frontend
```

### Example Configuration

```go
package main

import (
	"os/exec"
	"github.com/robo-monk/lid/lid"
)

func main() {

    manager := lid.New()

    manager.Register("pocketbase", &lid.Service{
        Cwd:      "../pocketbase",
        Command:  []string{"./pocketbase", "serve"},
        EnvFile:  ".env",
        OnExit:   func(e *exec.ExitError, service *lid.Service) {
		service.Logger.Println("pocketbase failed")
            	// ... log the error further
            	// Restart the service
           	service.Start()
        },
    })

    manager.Register("backend", &lid.Service {
        Cwd: 	 "../server"
      	EnvFile: ".production.env"
        Command: []string{"./dist/server"},
        OnExit:  func(e *exec.ExitError, service *lid.Service) {
           	service.Start()
        },
    })

    manager.Register("frontend", &lid.Service {
      	Cwd: 	 "../frontend"
        Command: []string{ "pnpm", "run", "start" },
        OnExit:  func(e *exec.ExitError, service *lid.Service) {
           	service.Start()
        },
    })

    manager.Run()
}
```

### Run it as a service
```
system-ctl enable --now path/to/lid/exe start
```
