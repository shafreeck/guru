package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shafreeck/guru/tui"
)

type record struct {
	Op  string
	Msg *Message
}

type history struct {
	w       io.ReadWriteCloser
	records []*record
}

func (h *history) append(op string, v *Message) error {
	r := &record{Op: op, Msg: v}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(h.w, string(data))
	if err != nil {
		return err
	}
	h.records = append(h.records, r)
	return nil
}

type session struct {
	mm      messageManager
	dir     string
	sid     string
	history history
}

func newSession(dir string) *session {
	return &session{dir: dir}
}

func (s *session) open(sid string) error {
	s.sid = sid
	if s.sid == "" {
		// open a new session
		now := time.Now()
		s.sid = fmt.Sprintf("chat-%d-%s", now.UnixMilli(), uuid.New())
	} else {
		// load the session
		if err := s.load(); err != nil {
			return err
		}
	}

	// open session file for appending
	f, err := os.OpenFile(path.Join(s.dir, s.sid), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	s.history.w = f
	return nil
}
func (s *session) close() {
	// nothing saved, delete the session
	if len(s.history.records) == 0 {
		os.Remove(path.Join(s.dir, s.sid))
	}
	if s.history.w != nil {
		s.history.w.Close()
	}
}

func (s *session) append(m *Message) {
	s.mm.append(m)
	if err := s.history.append(":append", m); err != nil {
		fmt.Println(red.Render(err.Error()))
	}
}

func (s *session) messages() []*Message {
	return s.mm.messages
}

func (s *session) load() error {
	f, err := os.Open(path.Join(s.dir, s.sid))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	scanner := bufio.NewScanner(f)
	var records []*record
	for scanner.Scan() {
		r := &record{}
		text := scanner.Bytes()
		if err := json.Unmarshal(text, r); err != nil {
			return err
		}
		records = append(records, r)
	}
	s.history.records = records
	s.replay(records)
	return nil
}

func (s *session) replay(records []*record) {
	render := &tui.JSONRenderer{}
	fmt.Println(blue.Render("replay session:", s.sid))
	for _, r := range records {
		args := strings.Fields(r.Op)
		fmt.Print(blue.Render(r.Op), " ")
		if r.Msg != nil {
			data, _ := json.Marshal(r.Msg)
			data, _ = render.Render(data)
			fmt.Println(string(data))
			args = append(args, "--role", string(r.Msg.Role), r.Msg.Content)
		} else {
			fmt.Println()
		}
		builtins.Launch(context.Background(), args)
	}
}

func (s *session) list() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		fmt.Println(red.Render(err.Error()))
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "chat-") {
			if entry.Name() == s.sid {
				fmt.Print(blue.Render("* "))
			} else {
				fmt.Print("  ")
			}
			fmt.Println(entry.Name())
		}
	}
}

// listen on builtin commands
func (s *session) onCommandEvent(args []string) {
	op := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(op, ":reset"):
		fallthrough
	case strings.HasPrefix(op, ":append"):
		fallthrough
	case strings.HasPrefix(op, ":message shrink"):
		fallthrough
	case strings.HasPrefix(op, ":message delete"):
		fallthrough
	case strings.HasPrefix(op, ":message append"):
		s.history.append(op, nil)
	}
}

func (s *session) listenOnBuiltins() {
	builtins.sess = s
}

func (s *session) switchCommand() {
	opts := struct {
		SID string `cortana:"sid"`
	}{}
	builtins.Parse(&opts)

	builtins.sess = nil
	s.mm.messages = nil // clear the messages
	s.history = history{}
	s.open(opts.SID)
	builtins.sess = s
}

func (s *session) registerCommands() {
	builtins.AddCommand(":session list", s.list, "list sessions")
	builtins.AddCommand(":session switch", s.switchCommand, "switch a session")
	s.mm.registerMessageCommands()
}
