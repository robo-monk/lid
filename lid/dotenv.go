package lid

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ReadDotEnvFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return []string{}, err
		// log.Fatalf("Could not read env file%v\n", err)
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
	return env, nil
}
