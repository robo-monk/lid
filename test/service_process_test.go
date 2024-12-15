package lid_test

import (
	"path/filepath"
	"testing"

	"github.com/robo-monk/lid/lid"
	"github.com/stretchr/testify/assert"
)

func TestServiceProcessReadWrite(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "service-process.lid")

	original := lid.ServiceProcess{
		Status: lid.RUNNING,
		Pid:    1234,
	}

	if err := original.WriteToFile(filename); err != nil {
		t.Fatalf("Failed to write service process: %v", err)
	}

	readBack, err := lid.ReadServiceProcess(filename)
	if err != nil {
		t.Fatalf("Failed to read service process: %v", err)
	}

	if readBack.Status != original.Status || readBack.Pid != original.Pid {
		t.Errorf("Mismatch in service process: got %v, want %v", readBack, original)
	}
}

// TestReadServiceProcessFileNotFound checks behavior when file doesn't exist.
func TestReadServiceProcessFileNotFound(t *testing.T) {
	t.Parallel()
	_, err := lid.ReadServiceProcess("nonexistent-file.lid")

	// assert.Equal(t, int64(0), sp.Pid, "Expect PID to be 0 (inactive)");
	// assert.Equal(t, lid.STOPPED, sp.Status, "Expected Process status to be STOPPED")
	assert.NotNil(t, err, "Expected error not to be nil")
}

// func TestReadServiceProcessCorruptFile(t *testing.T) {
// 	t.Parallel()
// 	tmpDir := t.TempDir()
// 	filename := filepath.Join(tmpDir, "corrupt.lid")

// 	if err := os.WriteFile(filename, []byte("not a valid struct"), 0644); err != nil {
// 		t.Fatal(err)
// 	}

// 	sp, err := lid.ReadServiceProcess(filename)

// 	assert.Equal(t, int64(0), sp.Pid, "Expect PID to be 0 (inactive)")
// 	assert.Equal(t, lid.STOPPED, sp.Status, "Expected Process status to be STOPPED")
// 	assert.Nil(t, err, "Expected error to be nil")
// }
