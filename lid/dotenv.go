package lid

import (
	"fmt"

	"github.com/joho/godotenv"
)

func ReadDotEnvFile(filename string) ([]string, error) {
	envMap, err := godotenv.Read(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env, nil
}
