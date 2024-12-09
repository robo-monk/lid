package lid_test

import (
	"path/filepath"
	"testing"

	"github.com/robo-monk/lid/lid"
	"github.com/stretchr/testify/assert"
)

// TestReadDotEnvFile checks reading well-formed env files.
func TestReadDotEnvFile_Valid(t *testing.T) {
	t.Parallel()
	filename := filepath.Join("testdata", "valid.env")
	env, err := lid.ReadDotEnvFile(filename)
	assert.Nil(t, err)
	expected := []string{"FOO=bar", "BAZ=qux"}

	assert.Equal(t, len(expected), len(env), "Env len mismatch")

	for i, line := range expected {
		assert.Equal(t, line, env[i])
	}
}

// TestReadDotEnvFile_Quoted tests removing quotes.
func TestReadDotEnvFile_Quoted(t *testing.T) {
	t.Parallel()
	filename := filepath.Join("testdata", "quoted.env")
	env, err := lid.ReadDotEnvFile(filename)

	assert.Nil(t, err)
	expected := []string{"FOO=hello world", "BAR= spaced value "}
	assert.Equal(t, len(expected), len(env), "Env len mismatch")
	for i, line := range expected {
		assert.Equal(t, line, env[i])
	}
}

// TestReadDotEnvFile_Malformed checks behavior with malformed env files.
// Here we mainly ensure it doesn't panic and returns partial results.
func TestReadDotEnvFile_Malformed(t *testing.T) {
	t.Parallel()
	filename := filepath.Join("testdata", "malformed.env")
	env, err := lid.ReadDotEnvFile(filename)
	assert.Nil(t, err)
	// We don't define strict behavior for malformed lines, just ensure no crash.
	if len(env) == 0 {
		t.Errorf("Expected some parsed environment variables, got none")
	}
}
