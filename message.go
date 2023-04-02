package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/shafreeck/guru/tui"
)

type messageManager struct {
	out      CommandOutput
	messages []*Message
}

func (m *messageManager) append(msg *Message) {
	m.messages = append(m.messages, msg)
}

func (m *messageManager) listCommand() (_ string) {
	opts := struct {
		N int `cortana:"--n, -n, 0, list the first n messages"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	render := &tui.JSONRenderer{}
	for i, msg := range m.messages {
		data, err := json.Marshal(msg)
		if err != nil {
			m.out.Errorln(err)
			return
		}
		text, err := render.Render(string(data))
		if err != nil {
			m.out.Errorln(err)
			return
		}
		m.out.Printf("%3d. %s", i, text)
		// we cat not use "\n" in Printf, it cause conficts with the style renderer
		m.out.Println()
	}
	return
}
func (m *messageManager) shrinkCommand() (_ string) {
	opts := struct {
		Expr string `cortana:"expr"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	var begin, end int
	var err error

	size := len(m.messages)

	parts := strings.Split(opts.Expr, ":")

	if v := parts[0]; v != "" {
		begin, err = strconv.Atoi(parts[0])
		if err != nil {
			m.out.Errorln(err)
		}
		if begin >= size {
			return
		}
	}
	if len(parts) == 1 {
		m.messages = m.messages[begin:]
		return
	}
	if v := parts[1]; v != "" {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			fmt.Fprintln(tui.Stderr, err)
		}
	} else {
		end = size
	}
	if end > size {
		end = size
	}
	m.messages = m.messages[begin:end]
	m.listCommand()
	return
}
func (m *messageManager) deleteCommand() (_ string) {
	opts := struct {
		Indexes []int `cortana:"index, -, -"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	for _, index := range opts.Indexes {
		if index < 0 || index >= len(m.messages) {
			continue
		}
		m.messages[index] = nil
	}
	var updated []*Message
	for _, msg := range m.messages {
		if msg != nil {
			updated = append(updated, msg)
		}
	}
	m.messages = updated
	return
}
func (m *messageManager) autoShrink() int {
	size := len(m.messages)
	switch size {
	case 0, 1:
		return 0
	case 2, 3:
		m.messages = m.messages[size-1:]
		return 1
	}
	idx := size / 2
	m.messages = m.messages[idx:]
	return idx
}

func (m *messageManager) showCommand() (_ string) {
	opts := struct {
		Indexes []int `cortana:"index, -"`
		Role    bool  `cortana:"--role, -r, false, show message with role"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	// nothing to show
	if len(m.messages) == 0 {
		return
	}

	// show the last message if no index supplied
	if len(opts.Indexes) == 0 {
		opts.Indexes = append(opts.Indexes, len(m.messages)-1)
	}
	out := bytes.NewBuffer(nil)
	for _, index := range opts.Indexes {
		if index < 0 || index >= len(m.messages) {
			continue
		}
		if opts.Role {
			out.WriteString(string(m.messages[index].Role) + ":\n\n")
		}
		out.WriteString(m.messages[index].Content + "\n\n")
	}
	tui.Display[tui.Model[string], string](context.Background(), tui.NewMarkdownModel(out.String()))
	return
}

func (m *messageManager) appendCommand() (_ string) {
	opts := struct {
		Role string `cortana:"--role, -r, user, append message with certain role"`
		Text string `cortana:"text"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	if opts.Text != "" {
		m.append(&Message{Role: ChatRole(opts.Role), Content: opts.Text})
	}
	return
}

func (m *messageManager) registerBuiltinCommands() {
	builtins.AddCommand(":message list", m.listCommand, "list messages")
	builtins.AddCommand(":message delete", m.deleteCommand, "delete messages")
	builtins.AddCommand(":message shrink", m.shrinkCommand, "shrink messages")
	builtins.AddCommand(":message show", m.showCommand, "show certain messages")
	builtins.AddCommand(":message append", m.appendCommand, "append a message")
	builtins.Alias(":ls", ":message list")
	builtins.Alias(":show", ":message show")
	builtins.Alias(":reset", ":message shrink 0:0")
	builtins.Alias(":append", ":message append")
}
