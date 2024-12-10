# lid

Lid is a very simple process manager inspired by pocketbase and pm2.
I use it in a monorepo to orchestrate various processes in production.

It aims to have:
- No background deamon
- Very simple interface
- Hooks to help orchestrate start up, and log crashes
- Zero(ish?) downtime when restarting applications

> young code use with care

### Recommended Installation
1.  Create a `lid` directory in the root of your monorepo
2.  Create a `lid/lig.go` and register your services
3. `go mod init lig && go mod tidy && go build -o lid`

### CLI Usage

```bash
lid CLI

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
    ...

```

### Example Configuration

```go
package main

import (
	"github.com/robo-monk/lid/lid"
	"os/exec"
)

func main() {

	manager := lid.New()

	manager.Register("pocketbase", &lid.Service{
		// the cwd is ALWAYS relative to the executable
		Cwd:     "../pocketbase",
		Command: []string{"./pocketbase", "serve"},

		// Env file relative to Cwd
		EnvFile: ".env",
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Logger.Println("pocketbase failed")

			// ... log the error further

			// Restart the service
			service.Start()
		},
	})

	manager.Register("backend", &lid.Service{
		// the cwd is ALWAYS relative to the executable
		Cwd: "../server",
		// Env file relative to Cwd
		EnvFile: ".production.env",
		Command: []string{"./dist/server"},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
			service.Start()
		},
	})

	manager.Register("frontend", &lid.Service{
		// the cwd is ALWAYS relative to the executable
		Cwd:     "../frontend",
		Command: []string{"pnpm", "run", "start"},
		OnExit: func(e *exec.ExitError, service *lid.Service) {
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
