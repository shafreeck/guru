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
	if err != nil {
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
	for _, r := range records {
		args := strings.Fields(r.Op)
		if r.Msg != nil {
			args = append(args, "--role", string(r.Msg.Role), r.Msg.Content)
		}
		builtins.Launch(context.Background(), args)
	}
}

func (s *session) registerCommands() {
	s.mm.registerMessageCommands()
}
