package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/c-bata/go-prompt"
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

func cmdCompleter(d prompt.Document) []prompt.Suggest {
	if d.LastKeyStroke() != prompt.Tab {
		return nil
	}

	line := strings.TrimLeft(d.CurrentLineBeforeCursor(), " ")
	if line == "" {
		return nil
	}

	// not a system command
	if line[0] != '$' {
		return nil
	}

	line = strings.TrimLeft(line[1:], " ")
	var suggests []prompt.Suggest
	// lookup the file in current path
	if strings.HasSuffix(line, " ") {
		var prefix string
		fields := strings.Fields(line)
		if len(fields) > 1 {
			prefix = fields[len(fields)-1]
		}
		entries, _ := os.ReadDir("./")
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				suggests = append(suggests, prompt.Suggest{Text: entry.Name()})
			}
		}
		return suggests
	}

	// lookup the commands in $PATH
	// TODO build an index for the lookup
	paths := strings.Split(os.Getenv("PATH"), ":")
	for _, path := range paths {
		entries, _ := os.ReadDir(path)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), line) {
				suggests = append(suggests, prompt.Suggest{Text: entry.Name()})
			}
		}
	}
	return suggests
}
