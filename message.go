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
	pinned   map[*Message]bool // pinned stores the index of pinned message
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
		if m.pinned[msg] {
			// It's not an error here, we leverage the red style
			m.out.Errorf("%3d. %s", i, text)
		} else {
			m.out.Printf("%3d. %s", i, text)
		}
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
		m.slice(begin, len(m.messages))
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
	m.slice(begin, end)
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

	// check pinned message first
	for _, index := range opts.Indexes {
		if index < 0 || index >= len(m.messages) {
			continue
		}
		if m.pinned[m.messages[index]] {
			m.out.Errorln(index, " is pinned, unpin it first")
			return
		}
	}

	// mark deleted message as nil
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
		m.slice(size-1, size)
		return 1
	}
	idx := size / 2
	m.slice(idx, size)
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

// slice the message with pinned consindered
func (m *messageManager) slice(begin, end int) {
	var header, tailer []*Message
	for i := 0; i < begin; i++ {
		if m.pinned[m.messages[i]] {
			header = append(header, m.messages[i])
		}
	}
	for i := end; i < len(m.messages); i++ {
		if m.pinned[m.messages[i]] {
			tailer = append(tailer, m.messages[i])
		}
	}

	messages := append(header, m.messages[begin:end]...)
	m.messages = append(messages, tailer...)
}

func (m *messageManager) pin(indexes ...int) {
	if m.pinned == nil {
		m.pinned = make(map[*Message]bool)
	}

	for _, index := range indexes {
		if index < 0 || index >= len(m.messages) {
			continue
		}
		m.pinned[m.messages[index]] = true
	}
}
func (m *messageManager) unpin(indexes ...int) {
	if m.pinned == nil {
		return
	}

	for _, index := range indexes {
		if index < 0 || index >= len(m.messages) {
			continue
		}
		delete(m.pinned, m.messages[index])
	}
}

func (m *messageManager) pinCommand() (_ string) {
	opts := struct {
		Indexes []int `cortana:"index, -, -"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}
	m.pin(opts.Indexes...)
	return
}

func (m *messageManager) unpinCommand() (_ string) {
	opts := struct {
		Indexes []int `cortana:"index, -, -"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	m.unpin(opts.Indexes...)

	return
}

func (m *messageManager) registerBuiltinCommands() {
	builtins.AddCommand(":message list", m.listCommand, "list messages")
	builtins.AddCommand(":message delete", m.deleteCommand, "delete messages")
	builtins.AddCommand(":message shrink", m.shrinkCommand, "shrink messages")
	builtins.AddCommand(":message show", m.showCommand, "show certain messages")
	builtins.AddCommand(":message append", m.appendCommand, "append a message")
	builtins.AddCommand(":message pin", m.pinCommand, "pin messages")
	builtins.AddCommand(":message unpin", m.unpinCommand, "unpin messages")
	builtins.Alias(":ls", ":message list")
	builtins.Alias(":show", ":message show")
	builtins.Alias(":reset", ":message shrink 0:0")
	builtins.Alias(":shrink", ":message shrink")
	builtins.Alias(":append", ":message append")
}
