# lid

Lid is a very simple process manager inspired by pocketbase and pm2.
I use it in a monorepo to orchestrate various processes in production.

It aims to have:
- No background deamon
- Very simple interface
- Callbacks when processes exit
- Zero(ish?) downtime when restarting applications

> young code use with care

## Usage
1. Create a `lid` directory in the root of your monorepo
2. Create a `lid/lid.go`

```go
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



    manager.Register("test", &lid.Service{
        Command: []string{"bash", "-c", "sleep 2; exit 1"},
        OnExit: func(e *exec.ExitError, service *lid.Service) {
           	service.Logger.Println("FAILED")
           	service.Start()
        },
    })

    manager.Run()
}
```

### Run it as a service
```
system-ctl enable --now cd <dir-with-lidfile> && lid start
```
