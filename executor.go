package main

import (
	"os/exec"
	"strings"
	"unicode"
)

type Executor struct {
	cmd string
}

func NewExecutor(cmd string) *Executor {
	return &Executor{cmd: cmd}
}

// Exec invokes the executor and feed input to
// stdin, and then return the stdout as a string
func (e *Executor) Exec(input string) (string, error) {
	args := splitQuoted(e.cmd)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// splitQuoted split s by spaces with keeping
// untouch on quoted fields
func splitQuoted(s string) []string {
	return strings.FieldsFunc(s, func() func(r rune) bool {
		arounded := false
		return func(r rune) bool {
			if r == '\'' || r == '"' {
				arounded = !arounded
				return true
			}
			if unicode.IsSpace(r) && !arounded {
				return true
			}
			return false
		}
	}())
}
