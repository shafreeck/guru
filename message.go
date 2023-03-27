package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
)

type messageManager struct {
	messages []*Message
}

func (m *messageManager) append(msg *Message) {
	m.messages = append(m.messages, msg)
}

func (m *messageManager) display() {
	opts := struct {
		N int `cortana:"--n, -n, 0, list the first n messages"`
	}{}
	builtins.Parse(&opts, cortana.IgnoreUnknownArgs())

	render := &tui.JSONRenderer{}
	for i, msg := range m.messages {
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
func (m *messageManager) shrink() {
	opts := struct {
		Expr string `cortana:"expr"`
	}{}
	builtins.Parse(&opts, cortana.IgnoreUnknownArgs())

	var begin, end int
	var err error

	size := len(m.messages)

	parts := strings.Split(opts.Expr, ":")

	if v := parts[0]; v != "" {
		begin, err = strconv.Atoi(parts[0])
		if err != nil {
			fmt.Println(err)
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
			fmt.Println(err)
		}
	} else {
		end = size
	}
	if end > size {
		end = size
	}
	m.messages = m.messages[begin:end]
	m.display()
}
func (m *messageManager) delete() {
	opts := struct {
		Indexes []int `cortana:"index, -"`
	}{}
	builtins.Parse(&opts, cortana.IgnoreUnknownArgs())

	// TODO make Indexes as a required argument
	if len(opts.Indexes) == 0 {
		builtins.Usage()
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

func (m *messageManager) show() {
	opts := struct {
		Indexes []int `cortana:"index, -"`
		Role    bool  `cortana:"--role, -r, false, show message with role"`
	}{}
	builtins.Parse(&opts, cortana.IgnoreUnknownArgs())
	// TODO make Indexes as a require argument
	if len(opts.Indexes) == 0 {
		builtins.Usage()
		return
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
}

func (m *messageManager) appendCommand() {
	opts := struct {
		Role string `cortana:"--role, -r, user, append message with certain role"`
		Text string `cortana:"text"`
	}{}
	builtins.Parse(&opts)
	if opts.Text != "" {
		m.append(&Message{Role: ChatRole(opts.Role), Content: opts.Text})
	}
}

func (m *messageManager) registerMessageCommands() {
	builtins.AddCommand(":message list", m.display, "list messages")
	builtins.AddCommand(":message delete", m.delete, "delete messages")
	builtins.AddCommand(":message shrink", m.shrink, "shrink messages")
	builtins.AddCommand(":message show", m.show, "show certain messages")
	builtins.AddCommand(":message append", m.appendCommand, "append a message")
	builtins.Alias(":list", ":message list")
	builtins.Alias(":show", ":message show")
	builtins.Alias(":reset", ":message reset")
	builtins.Alias(":append", ":message append")

}
