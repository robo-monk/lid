package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

type ExitEvent struct {
	code int
}

type Service struct {

	name 	string
	cwd		string
	command []string

	envFile	string

	onExit 	func(ok bool, e *exec.ExitError)
}

func readDotEnvFile(filename string) []string {
	log.Println("FILENAME: ", filename)
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Could not read env file%v\n", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	env := []string {}

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(strings.Join(parts[1:], ""))

		// trim '"'
		if len(value) > 0 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1:len(value)-1]
		}

		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func (s *Service) GetPidFilename() string {
	return fmt.Sprintf("/tmp/lid-%s.pid", s.name)
}


func (s *Service) getProcess() (*process.Process, error) {
	pidContent, fileErr := os.ReadFile(s.GetPidFilename())

	if fileErr != nil {
		if os.IsNotExist(fileErr) {
			return nil, ErrProcessNotFound
		} else {
			// log.Printf("%s	| CORRUPT  (%s)\n", service.name, fileErr.Error())
			return nil, ErrProcessCorrupt
		}
	} else {
		pid, err := strconv.Atoi(string(pidContent))

		if err != nil {
			return nil, ErrProcessCorrupt
		}

		p, err  := process.NewProcess(int32(pid))

		if err != nil {
			os.Remove(s.GetPidFilename());
			return nil, ErrProcessNotFound
		}

		return p, nil
	}
}

func (s *Service) Start() {

	fmt.Printf("Starting service %s\n", s.name);
	fmt.Println("Starting command..")

	cmd := exec.Command(s.command[0], s.command[1:]...)

	if s.cwd != "" {
		cmd.Dir, _ = filepath.Abs(s.cwd)
	}

	if s.envFile != "" {
		cmd.Env = readDotEnvFile(filepath.Join(s.cwd, s.envFile))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the command in the background
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start command: %v", err)
	}

	// Get the PID of the background process
	log.Printf("Started process with PID: %d", cmd.Process.Pid)
	pid := cmd.Process.Pid;
	filename := s.GetPidFilename()
	os.WriteFile(filename, []byte(strconv.Itoa(pid)), 0666)
	log.Printf("Registering PID file '%s'\n", filename)

	err = cmd.Wait()

	if err != nil {
		log.Printf("Command exited with error: %v", err)
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		s.onExit(false, exitErr)
	} else {
		s.onExit(true, nil)
	}

	log.Printf("Command exited\n")
}

func (s *Service) Stop() error {
	process, err := s.getProcess()
	if err != nil {
		log.Printf("Service '%s' is already down\n", s.name)
		return err
	}

	log.Printf("Stopping service '%s'\n", s.name)
	recursiveKill(process)
	filename := s.GetPidFilename()
	os.Remove(filename)
	return nil
}

var (
	ErrProcessNotFound = fmt.Errorf("Process Not Found")
	ErrProcessCorrupt = fmt.Errorf("Process Not Found")
)

func recursiveKill(p *process.Process) {
	children, _ := p.Children()
	for _, child := range children {
		recursiveKill(child)
	}
	p.Kill()
}
