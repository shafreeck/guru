package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
)

// the builtin commands

type CommandListener interface {
	OnCommand(args []string) // args[0] is the command name
}

type BuiltinCommand struct {
	*cortana.Cortana

	completes *Completion
	listeners map[CommandListener]struct{}
	text      string
}

func (c *BuiltinCommand) Launch(args []string) string {
	cmd := c.SearchCommand(args)
	if cmd == nil {
		usage := lipgloss.NewStyle().Foreground(
			lipgloss.AdaptiveColor{Dark: "#79b3ec", Light: "#1d73c9"}).
			Render(c.UsageString())
		fmt.Fprint(tui.Stdout, usage)
		return ""
	}
	cmd.Proc()
	text := c.text
	c.text = "" // clear the state

	for l := range builtins.listeners {
		l.OnCommand(args)
	}
	return text
}

func (c *BuiltinCommand) AddListener(l CommandListener) {
	c.listeners[l] = struct{}{}
}
func (c *BuiltinCommand) RemoveListener(l CommandListener) {
	delete(c.listeners, l)
}

// Parse the args and set usgae if meet --help/-h
func (c *BuiltinCommand) Parse(v interface{}) (usage bool) {
	c.Cortana.Parse(v, cortana.OnUsage(func(usageString string) {
		c.Usage()
		usage = true
	}))
	return
}

type BuiltinCommandFunc func() string

func (c *BuiltinCommand) AddCommand(path string, cmd BuiltinCommandFunc,
	breif string, complete ...CompleteCommand) {
	c.Cortana.AddCommand(path, builtin(cmd), breif)
	if len(complete) == 0 {
		c.completes.AddCommand(path, func(s []rune, pos int) ([][]rune, int) { return nil, 0 })
		return
	}
	c.completes.AddCommand(path, complete[0])
}
func (c *BuiltinCommand) Alias(name, definition string) {
	c.Cortana.Alias(name, definition)
	c.completes.Alias(name, definition)
}

func builtin(f func() string) func() {
	return func() {
		builtins.text = f()
	}
}

var builtins = BuiltinCommand{Cortana: cortana.New(cortana.ExitOnError(false)),
	completes: completes, listeners: make(map[CommandListener]struct{})}

func init() {
	// add commands
	builtins.AddCommand(":exit", exit, "exit guru")
	builtins.AddCommand(":read", read, "read from stdin with a textarea")
	builtins.AddCommand(":help", func() string {
		return builtins.Launch(nil) // launch none commonds, so it print usage and return ""
	}, "help for commands")

	// add aliases
	builtins.Alias(":quit", ":exit")

	// add completion handler for system commands
	completes.AddCommand("$", sysCommandComplete)
}

func exit() string {
	os.Exit(0)
	return ""
}

func read() string {
	opts := struct {
		Prompt []string `cortana:"prompt"`
	}{}
	builtins.Parse(&opts)

	// no error in this model
	text, _ := tui.Display[tui.Model[string], string](context.Background(), tui.NewTextAreaModel())
	return strings.Join(opts.Prompt, " ") + "\n" + text
}
