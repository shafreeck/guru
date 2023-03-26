package main

import (
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/shafreeck/cortana"
)

// the builtin commands

type builtinCommand struct {
	*cortana.Cortana
}

var builtins = builtinCommand{Cortana: cortana.New()}

func init() {
	builtins.AddCommand(":exit", exit, "exit guru")
	builtins.Alias(":quit", ":exit")
}

func exit() {
	os.Exit(0)
}

func builtinCompleter(d prompt.Document) []prompt.Suggest {
	prefix := strings.TrimSpace(d.CurrentLineBeforeCursor())
	cmds := builtins.Complete(prefix)
	var suggests []prompt.Suggest
	for _, cmd := range cmds {
		suggests = append(suggests, prompt.Suggest{
			Text: cmd.Path,
		})
	}
	return suggests
}
