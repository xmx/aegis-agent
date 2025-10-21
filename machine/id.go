package machine

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ID returns the platform specific machine id of the current host OS.
// Regard the returned id as "confidential" and consider using ProtectedID() instead.
func ID() (string, error) {
	id, err := machineID()
	if err != nil {
		return "", fmt.Errorf("machineid: %v", err)
	}
	sum := sha1.Sum([]byte(id))
	mid := hex.EncodeToString(sum[:])

	return mid, nil
}

// run wraps `exec.Command` with easy access to stdout and stderr.
func run(stdout, stderr io.Writer, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = stdout
	c.Stderr = stderr
	return c.Run()
}

func readFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func trim(s string) string {
	return strings.TrimSpace(strings.Trim(s, "\n"))
}
