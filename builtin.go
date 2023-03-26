package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
)

// the builtin commands

type builtinCommand struct {
	*cortana.Cortana
}

func (c *builtinCommand) Launch(args ...string) {
	cmd := c.SearchCommand(args)
	if cmd == nil {
		usage := lipgloss.NewStyle().Foreground(
			lipgloss.AdaptiveColor{Dark: "#79b3ec", Light: "#1d73c9"}).
			Render(c.UsageString())
		fmt.Println(usage)
		return
	}
	cmd.Proc()
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
