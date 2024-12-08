# lid

```go
// lid.go
func main() {

	lid.Register("pocketbase", Service {
		cwd: 	"../../convex/convex/pocketbase"
		cmd: 	`dotenv -- ./convex-pb serve`
		onFail: func (e *FailEvent, restart *Restart) {
			logToTelgram(e)
			restart()
		}
	})

	lid.Register("convex-insights-server", Service {
		cwd: 	"../../convex/convex/convex-insights/server"
		cmd: 	`dotenv -- ./dist/server`
		onFail: func (e *FailEvent, restart *Restart) {
			logToTelgram(e)
			restart()
		}
	})
}
```

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
