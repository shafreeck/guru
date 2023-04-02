package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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

func sysCommandComplete(line []rune, pos int) ([][]rune, int) {
	prefix := string(line)
	prefix = strings.TrimLeft(prefix[1:], " ")

	var suggests [][]rune
	fields := strings.Fields(prefix)
	// lookup the file in current path
	if strings.HasSuffix(prefix, " ") || len(fields) > 1 {
		var p string
		if len(fields) > 1 && !strings.HasSuffix(prefix, " ") {
			p = fields[len(fields)-1]
		}
		entries, _ := os.ReadDir("./")
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), p) {
				suggests = append(suggests, []rune(strings.TrimPrefix(entry.Name(), p)))
			}
		}
		return suggests, len(p)
	}

	// lookup the commands in $PATH
	paths := strings.Split(os.Getenv("PATH"), ":")
	for _, path := range paths {
		entries, _ := os.ReadDir(path)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				suggests = append(suggests, []rune(strings.TrimPrefix(entry.Name(), prefix)))
			}
		}
	}
	return suggests, pos - 1 // remove the $
}
