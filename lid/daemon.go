package lid

// import (
// 	"context"
// 	"fmt"
// 	"log/slog"
// 	"net"
// 	"net/rpc"
// 	"os"
// 	"os/signal"
// 	"path/filepath"
// 	"sync"
// 	"syscall"
// )

// type Daemon struct {
// 	listener   net.Listener
// 	server     *rpc.Server
// 	supervisor *Supervisor
// 	ctx        context.Context
// 	cancel     context.CancelFunc
// }

// type Supervisor struct {
// 	services map[string]*Service
// 	mu       sync.RWMutex
// }

// func NewDaemon() (*Daemon, error) {
// 	ctx, cancel := context.WithCancel(context.Background())

// 	supervisor := &Supervisor{
// 		services: make(map[string]*Service),
// 	}

// 	server := rpc.NewServer()
// 	server.RegisterName("Lid", supervisor)

// 	// socket
// 	socketPath := filepath.Join(os.TempDir(), "lid.sock")
// 	os.Remove(socketPath)
// 	listener, err := net.Listen("unix", socketPath)
// 	if err != nil {
// 		cancel()
// 		return nil, fmt.Errorf("failed to listen on socket: %w", err)
// 	}

// 	return &Daemon{
// 		listener:   listener,
// 		server:     server,
// 		supervisor: supervisor,
// 		ctx:        ctx,
// 		cancel:     cancel,
// 	}, nil
// }

// func (d *Daemon) Run() error {
// 	defer d.listener.Close()

// 	sigCh := make(chan os.Signal, 1)
// 	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

// 	go func() {
// 		for {
// 			conn, err := d.listener.Accept()
// 			if err != nil {
// 				select {
// 				case <-d.ctx.Done():
// 					return
// 				default:
// 					slog.Error("failed to accept connection", "error", err)
// 					continue
// 				}
// 			}

// 			go d.server.ServeConn(conn)
// 		}
// 	}()

// 	d.supervisor.MonitorServices(d.ctx)
// 	<-sigCh
// 	d.Shutdown()
// 	return nil
// }

// func (d *Daemon) Shutdown() {
// 	d.cancel()
// 	d.listener.Close()
// }
