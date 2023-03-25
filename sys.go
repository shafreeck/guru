package main

import (
	"fmt"
	"os/exec"
)

// run sys command in the interactive mode

func runCommand(line string) (string, error) {
	cmd := exec.Command("sh", "-c", line)
	output, err := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		return "", fmt.Errorf("%s", string(output))
	}
	return string(output), err
}
