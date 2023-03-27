package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
)

// the builtin commands

type builtinCommand struct {
	*cortana.Cortana

	sess *session
	ctx  context.Context
	text string
}

func (c *builtinCommand) Launch(ctx context.Context, args []string) string {
	cmd := c.SearchCommand(args)
	if cmd == nil {
		usage := lipgloss.NewStyle().Foreground(
			lipgloss.AdaptiveColor{Dark: "#79b3ec", Light: "#1d73c9"}).
			Render(c.UsageString())
		fmt.Println(usage)
		return ""
	}
	c.ctx = ctx
	cmd.Proc()
	text := c.text
	c.text = "" // clear the state
	if c.sess != nil {
		c.sess.onCommandEvent(args)
	}
	return text
}

func builtin(f func(ctx context.Context) string) func() {
	return func() {
		builtins.text = f(builtins.ctx)
	}
}

var builtins = builtinCommand{Cortana: cortana.New()}

func init() {
	builtins.AddCommand(":exit", exit, "exit guru")
	builtins.Alias(":quit", ":exit")
	builtins.AddCommand(":read", builtin(read), "read from stdin with a textarea")
}

func builtinCompleter(d prompt.Document) []prompt.Suggest {
	prefix := strings.TrimLeft(d.CurrentLineBeforeCursor(), " ")
	cmds := builtins.Complete(prefix)
	var suggests []prompt.Suggest
	for _, cmd := range cmds {
		path := cmd.Path
		fields := strings.Fields(prefix)
		if strings.HasSuffix(prefix, " ") || len(fields) > 1 {
			path = strings.TrimSpace(strings.TrimPrefix(cmd.Path, strings.TrimSpace(fields[0])))
		}
		if path == "" {
			continue
		}
		suggests = append(suggests, prompt.Suggest{
			Text: path,
		})
	}
	return suggests
}

func exit() {
	os.Exit(0)
}

func read(ctx context.Context) string {
	opts := struct {
		Prompt []string `cortana:"prompt"`
	}{}
	builtins.Parse(&opts)

	// no error in this model
	text, _ := tui.Display[tui.Model[string], string](ctx, tui.NewTextAreaModel())
	return strings.Join(opts.Prompt, " ") + "\n" + text
}
