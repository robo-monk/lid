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
import (
	"github.com/robo-monk/lid/lid"
)

func main() {
	m := lid.New()
	m.Register("pocketbase", &lid.Service {
		cwd: "../pocketbase",
		cmd: "./pocketbase serve"
		envFile: ".env",
		onExit: func (e *exec.ExitError, s *lid.Service) {
			if e != nil {
				lid.logger.Printf("Pocketbase exited with bad error: %v\n", e)
			}

			restart()
		}
	})
}
```


```go
// lid.go
func main() {

	lid.Register("pocketbase", Service {
		cwd: 	"../../convex/convex/pocketbase"
		cmd: 	`dotenv -- ./convex-pb serve`
		onExit: func (e *ExitEvent, restart *Restart) {
			logToTelgram(e)
			restart()
		}
	})

	lid.Register("convex-insights-server", Service {
		cwd: 	"../../convex/convex/convex-insights/server"
		cmd: 	`dotenv -- ./dist/server`
		onExit: func (e *ExitEvent, restart *Restart) {
			logToTelgram(e)
			restart()
		}
	})
}
```
`lid start pocketbase`
lid --event start pocketbase
echo "start pocketbase"
cmd="sleep 100; exit 42";
echo "Command: $cmd";
(eval "$cmd");
exit_code=$?;
echo "Exit Code: $exit_code"
echo "exit pocketbase"
lid --event exited pocketbase $exit_code


```
# Lidfile

@start pocketbase
  cd ../../convex/convex/pocketbase
  dotenv -- ./convex-pb serve

@on_event
  ./logger.ts
```

lid start pocketbase
--> spawn pocketbase

### run it as a service
```
system-ctl enable --now cd <dir-with-lidfile> && lid start
```

```
lid start convex-forms
```

```
lid restart convex-forms
```

```
lid stop convex-forms
```

```
lid attach convex-forms
```

### callbacks if the service goes down

### lid.go
import (
    "github.com/lid
)

func main() {
    lid.Register("convex-forms", {
        start: ""
    })
}


convex-forms:
    start
