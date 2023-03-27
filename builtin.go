package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
)

// the builtin commands

type builtinCommand struct {
	*cortana.Cortana

	ctx  context.Context
	text string
}

func (c *builtinCommand) Launch(ctx context.Context, args ...string) string {
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

type messageManager struct {
}

func (m *messageManager) display(messages []*Message) {
	opts := struct {
		N int `cortana:"--n, -n, 0, list the first n messages"`
	}{}
	builtins.Parse(&opts, cortana.IgnoreUnknownArgs())

	render := &tui.JSONRenderer{}
	for i, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			fmt.Println(err)
		}
		text, err := render.Render(data)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(fmt.Sprintf("%3d. ", i), string(text))
	}
}
func (m *messageManager) shrink(messages []*Message) []*Message {
	opts := struct {
		Expr string `cortana:"expr"`
	}{}
	builtins.Parse(&opts, cortana.IgnoreUnknownArgs())

	var begin, end int
	var err error

	size := len(messages)

	parts := strings.Split(opts.Expr, ":")

	if v := parts[0]; v != "" {
		begin, err = strconv.Atoi(parts[0])
		if err != nil {
			fmt.Println(err)
		}
		if begin >= size {
			return messages
		}
	}
	if len(parts) == 1 {
		return messages[begin:]
	}
	if v := parts[1]; v != "" {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			fmt.Println(err)
		}
	} else {
		end = size
	}
	if end > size {
		end = size
	}
	return messages[begin:end]
}
func (m *messageManager) delete(messages []*Message) []*Message {
	opts := struct {
		Indexes []int `cortana:"index, -, -"`
	}{}
	builtins.Parse(&opts, cortana.IgnoreUnknownArgs())

	for _, index := range opts.Indexes {
		if index < 0 || index > len(messages) {
			continue
		}
		messages[index] = nil
	}
	var updated []*Message
	for _, msg := range messages {
		if msg != nil {
			updated = append(updated, msg)
		}
	}
	return updated
}
func (m *messageManager) autoShrink(messages []*Message) (int, []*Message) {
	size := len(messages)
	switch size {
	case 0, 1:
		return 0, messages
	case 2, 3:
		return 1, messages[size-1:]
	}
	idx := size / 2
	return idx, messages[idx:]
}
