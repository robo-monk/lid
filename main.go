package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"github.com/shirou/gopsutil/v4/process"
)

type ExitEvent struct {
	code int
}
type Service struct {

	name 	string
	cwd		string
	cmd		string

	onExit 	func(e ExitEvent)
}


type Lid struct {
	services []Service
}

func (lid *Lid) Register(serviceName string, s Service) {
	s.name = serviceName;
	lid.services = append(lid.services, s)
}

func wrap(s Service) string {
	lidExec, _ := os.Executable()
	cmd := ""
	cmd += fmt.Sprintf("%s --event start %s;", lidExec, s.name)
	cmd += fmt.Sprintf("(%s);", s.cmd)
	cmd += "exit_code=$?;"
	cmd += fmt.Sprintf("%s --event exited %s $exit_code", lidExec, s.name)
	fmt.Println(cmd)
	return cmd
}

func (s Service) getPidFilename() string {
	return fmt.Sprintf("/tmp/lid-%s.pid", s.name)
}

const invalidUsage = ("Invalid usage. Usage lid start | status")
func main() {
	// stop := make(chan os.Signal, 1)
	//

	executablePath, _ := os.Executable();
	fmt.Println("os.Executable", executablePath);
	fmt.Println(len(os.Args), os.Args)



	if len(os.Args) < 2 {
		panic(invalidUsage)
	}

	lid := Lid {}

	lid.Register("pocketbase", Service {
		cmd: 	`cd ../../convex/convex/pocketbase && dotenv -- ./convex-pb serve`,
		onExit: func (e ExitEvent) {
			fmt.Printf("Process exited\n")
		},
	});


	lid.Register("test", Service {
		cmd: 	`sleep 40; exit 42`,
		onExit: func (e ExitEvent) {
			fmt.Printf("Process exited\n")
		},
	});


	// switch os.Args[1]

	// data, err := os.ReadFile("Lidfile")
	// if err != nil {
	// 	panic("Lidfile not found in cwd.")
	// }

	// println("Lidfile", string(data))

	// services := ParseLidfile(string(data))

	switch os.Args[1] {
	case "start":
		for _, service := range lid.services {

			pidFile, error := os.ReadFile(service.getPidFilename())

			if error == nil {
				log.Printf("service '%s' is already running with PID %s \n", service.name, pidFile)
				continue
			}

			fmt.Printf("Starting service %s\n", service.name);

			fmt.Println("Starting command..")
			cmd := exec.Command("bash", "-c", wrap(service))
			// cmd.Stdout = os.Stdout
			// cmd.Stderr = os.Stderr

			// Start the command in the background
			err := cmd.Start()
			if err != nil {
				log.Fatalf("Failed to start command: %v", err)
			}

			// Get the PID of the background process
			log.Printf("Started process with PID: %d", cmd.Process.Pid)

			pid := cmd.Process.Pid;

			// Detach from the process (if needed)
			err = cmd.Process.Release()
			if err != nil {
				log.Fatalf("Failed to release process: %v", err)
			}

			log.Println("Process detached. Exiting main program.")

			filename := service.getPidFilename()
			os.WriteFile(filename, []byte(strconv.Itoa(pid)), 0666)
			log.Printf("Registering PID file '%s'\n", filename)
		}
	case "list":
		for _, service := range lid.services {
			pidContent, fileErr := os.ReadFile(service.getPidFilename())

			if fileErr != nil {
				if os.IsNotExist(fileErr) {
					log.Printf("%s	| DOWN\n", service.name)
				} else {
					log.Printf("%s	| CORRUPT  (%s)\n", service.name, fileErr.Error())
				}
			} else {
				pid, err := strconv.Atoi(string(pidContent))

				if err != nil {
					log.Printf("%s	| CORRUPT  (%s)\n", service.name, err.Error())
					continue
				}

				p, err  := process.NewProcess(int32(pid))

				if err != nil {
					os.Remove(service.getPidFilename());
					log.Printf("%s	| DOWN\n", service.name)
					continue
				}

				createTime, _ := p.CreateTime()
				upTime := time.Now().UnixMilli() - createTime
				cpu, _ := p.CPUPercent()

				log.Printf("%s	| RUNNING (%d mins) [%f%%]\n", service.name, (upTime / 1000 / 60), cpu)

			}
		}
	case "--event":
		if err := os.WriteFile("out.txt", []byte(strings.Join(os.Args, ", ")), os.ModeAppend); err != nil {
        	fmt.Println(err)
		}
	default:
		panic(invalidUsage)
	}

}
