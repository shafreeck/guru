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

func cmdCompleter(prefix []rune, pos int) ([][]rune, int) {
	line := string(prefix)
	line = strings.TrimLeft(line, " ")
	if line == "" {
		return nil, 0
	}

	// not a system command
	if line[0] != '$' {
		return nil, 0
	}

	line = strings.TrimLeft(line[1:], " ")

	var suggests [][]rune
	fields := strings.Fields(line)
	// lookup the file in current path
	if strings.HasSuffix(line, " ") || len(fields) > 1 {
		var prefix string
		if len(fields) > 1 {
			prefix = fields[len(fields)-1]
		}
		entries, _ := os.ReadDir("./")
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				suggests = append(suggests, []rune(strings.TrimPrefix(entry.Name(), prefix)))
			}
		}
		return suggests, len(prefix)
	}

	// lookup the commands in $PATH
	paths := strings.Split(os.Getenv("PATH"), ":")
	for _, path := range paths {
		entries, _ := os.ReadDir(path)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), line) {
				suggests = append(suggests, []rune(strings.TrimPrefix(entry.Name(), line)))
			}
		}
	}
	return suggests, pos - 1 // remove the $
}
