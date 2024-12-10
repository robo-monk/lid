package lid

import "fmt"

var (
	ErrProcessNotFound       = fmt.Errorf("process not found")
	ErrProcessCorrupt        = fmt.Errorf("process corrupt")
	ErrProcessAlreadyRunning = fmt.Errorf("service is already running")
)
