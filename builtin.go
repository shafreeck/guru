package main

import (
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

func exit() {
	os.Exit(0)
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
	}
	if len(parts) == 1 && begin < size {
		return messages[begin:]
	}
	if v := parts[0]; v != "" {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			fmt.Println(err)
		}
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
