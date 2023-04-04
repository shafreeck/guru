package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/shafreeck/guru/tui"
)

type record struct {
	Op     string
	Msg    *Message
	Offset int64
}

type history struct {
	offset  int64 // the offset of the write cursor
	w       io.WriteCloser
	records []*record
}

func (h *history) Write(data []byte) (n int, err error) {
	n, err = h.w.Write(data)
	h.offset += int64(n)
	return n, err
}

func (h *history) append(op string, v *Message) error {
	r := &record{Op: op, Msg: v, Offset: h.offset}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(h, string(data))
	if err != nil {
		return err
	}
	h.records = append(h.records, r)
	return nil
}

type Session struct {
	out       CommandOutput
	mm        messageManager
	highlight lipgloss.Style
	dir       string
	sid       string
	stack     []string
	stackOnce sync.Once
	history   history
}

type SessionOption func(s *Session)

func WithCommandOutput(out CommandOutput) SessionOption {
	return func(s *Session) {
		s.out = out
	}
}

func WithHighlightStyle(style lipgloss.Style) SessionOption {
	return func(s *Session) {
		s.highlight = style
	}
}

func NewSession(dir string, opts ...SessionOption) *Session {
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("#2da9d2"))

	s := &Session{
		dir:       dir,
		out:       &commandStdout{},
		highlight: blue,
	}

	for _, opt := range opts {
		opt(s)
	}

	// Use session's CommandOutput
	s.mm.out = s.out
	// Register the session and message commands
	s.registerBuiltinCommands()

	return s
}

func (s *Session) Open(sid string) error {
	builtins.RemoveListener(s)
	defer builtins.AddListener(s)
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
	info, err := f.Stat()
	if err != nil {
		return err
	}
	s.history.w = f
	s.history.offset = info.Size()
	s.stackOnce.Do(func() {
		s.stack = append(s.stack, s.sid)
	})
	return nil
}

func (s *Session) Remove(sid string) error {
	return os.Remove(path.Join(s.dir, sid))
}

func (s *Session) Close() {
	// nothing saved, delete the session
	if len(s.history.records) == 0 {
		s.Remove(s.sid)
	}
	if s.history.w != nil {
		s.history.w.Close()
	}
}

func (s *Session) Append(m *Message) {
	s.mm.append(m)
	if err := s.history.append(":append", m); err != nil {
		s.out.Errorln(err)
	}
}

func (s *Session) Messages() []*Message {
	return s.mm.messages
}

func (s *Session) load() error {
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
	if len(records) > 0 {
		s.history.records = records
		s.replay(records)
	}
	return nil
}

func (s *Session) replay(records []*record) {
	render := &tui.JSONRenderer{}
	s.out.Println("replay session:", s.sid)
	for _, r := range records {
		args := strings.Fields(r.Op)
		s.out.Print(r.Op, " ")
		if r.Msg != nil {
			data, err := json.Marshal(r.Msg)
			if err != nil {
				s.out.Errorln(err)
				return
			}
			text, err := render.Render(string(data))
			if err != nil {
				s.out.Errorln(err)
				return
			}

			s.out.Println(text)
			args = append(args, "--role", string(r.Msg.Role), r.Msg.Content)
		} else {
			s.out.Println()
		}
		builtins.Launch(args)
	}
}

func (s *Session) listCommand() (_ string) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		s.out.Errorln(err)
	}
	for i, entry := range entries {
		if entry.Name() == s.sid {
			s.out.StylePrintf(s.highlight, "  *  %s", entry.Name())
			s.out.Println()
			continue
		}
		s.out.Printf("%3d. ", i)
		s.out.Println(entry.Name())
	}
	return
}

// listen on builtin commands
func (s *Session) OnCommand(args []string) {
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

func (s *Session) switchSession(sid string) {
	s.Close()
	s.mm.messages = nil // clear the messages
	s.history = history{}
	s.Open(sid)
}

func (s *Session) switchCommand() (_ string) {
	opts := struct {
		SID string `cortana:"sid"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	if opts.SID == "" {
		builtins.Usage()
		return
	}

	if _, err := os.Stat(path.Join(s.dir, opts.SID)); err != nil {
		s.out.Errorln(err)
		return
	}

	s.switchSession(opts.SID)
	return
}

func (s *Session) removeCommand() (_ string) {
	opts := struct {
		SID string `cortana:"sid"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return
	}

	if opts.SID == "" {
		builtins.Usage()
		return
	}
	if err := s.Remove(opts.SID); err != nil {
		s.out.Errorln(err)
	}
	return
}

func (s *Session) newCommand() string {
	opts := struct {
		SID   string   `cortana:"--session-id, -s, ,the session id"`
		Texts []string `cortana:"text"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return ""
	}

	if opts.SID != "" {
		if _, err := os.Stat(path.Join(s.dir, opts.SID)); err == nil {
			s.out.Errorln("session \"" + opts.SID + "\" exist")
			return ""
		}
	}

	s.switchSession(opts.SID)
	s.out.Println("session " + s.sid + " created")

	return strings.Join(opts.Texts, " ")
}

func (s *Session) shrinkCommand() (_ string) {
	opts := struct {
		Expr string `cortana:"expr"`
	}{}

	if usage := builtins.Parse(&opts); usage {
		return
	}

	var begin, end int
	var err error

	var ids []string
	var removes []string

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		s.out.Errorln(err)
		return
	}
	for _, entry := range entries {
		ids = append(ids, entry.Name())
	}

	size := len(ids)

	parts := strings.Split(opts.Expr, ":")

	if v := parts[0]; v != "" {
		begin, err = strconv.Atoi(parts[0])
		if err != nil {
			s.out.Errorln(err)
			return
		}
		if begin >= size {
			return
		}
	}
	if len(parts) == 1 {
		removes = ids[:begin]
	} else {
		if v := parts[1]; v != "" {
			end, err = strconv.Atoi(parts[1])
			if err != nil {
				s.out.Errorln(err)
				return
			}
		} else {
			end = size
		}
		if end > size {
			end = size
		}
		removes = append(ids[0:begin], ids[end:]...)
	}

	for _, sid := range removes {
		if sid == s.sid {
			continue
		}
		s.Remove(sid)
		s.out.Println(sid + " removed")
	}
	return
}

func (s *Session) historyCommand() (_ string) {
	render := tui.JSONRenderer{}
	for i, record := range s.history.records {
		data, err := json.Marshal(record)
		if err != nil {
			s.out.Errorln(err)
			return
		}
		text, err := render.Render(string(data))
		if err != nil {
			s.out.Errorln(err)
			return
		}
		s.out.Printf("%3d. %s", i, text)
		s.out.Println()
	}
	return
}
func (s *Session) stackPushCommand() string {
	opts := struct {
		SID   string   `cortana:"--session-id, -s, ,the session id"`
		Texts []string `cortana:"text"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return ""
	}

	// switch to the session
	s.switchSession(opts.SID)
	s.stack = append(s.stack, s.sid)
	s.out.Println("step in session: " + s.sid)

	return strings.Join(opts.Texts, " ")
}
func (s *Session) stackPopCommand() (_ string) {
	size := len(s.stack)
	// left the current session
	if size == 1 {
		return
	}

	s.stack = s.stack[:size-1]

	size--
	if len(s.stack) > 0 {
		s.switchSession(s.stack[size-1])
	}
	return
}
func (s *Session) stackShowCommand() (_ string) {
	out := bytes.NewBuffer(nil)
	total := len("chat-1680190522751-43e17bad-c0f1-4e2d-9902-db5d1ce965d2")
	truncated := len("chat-1680190522751-43e17bad-c0f1-4e2d-9902-")
	for _, sid := range s.stack {
		// use a short format to make it look friendly
		// notice that the short ids may collide. use :session list to
		// find the original id
		if len(sid) == total {
			sid = sid[truncated:]
		}
		out.WriteString(fmt.Sprintf(" > %s", sid))
	}
	s.out.Println(out.String())
	return
}

func (s *Session) registerBuiltinCommands() {
	builtins.AddCommand(":session new", s.newCommand, "create a new session")
	builtins.AddCommand(":session remove", s.removeCommand, "delete a session")
	builtins.AddCommand(":session shrink", s.shrinkCommand, "shrink sessions")
	builtins.AddCommand(":session list", s.listCommand, "list sessions")
	builtins.AddCommand(":session switch", s.switchCommand, "switch a session")
	builtins.AddCommand(":session history", s.historyCommand, "print history of current session")
	builtins.AddCommand(":session stack", s.stackShowCommand, "show the session stack")
	builtins.AddCommand(":session stack push", s.stackPushCommand, "create a new session, and stash the current")
	builtins.AddCommand(":session stack pop", s.stackPopCommand, "pop out current session")

	builtins.Alias(":new", ":session new")
	builtins.Alias(":stack", ":session stack")
	builtins.Alias(":switch", ":session switch")
	builtins.Alias(">", ":session stack push")
	builtins.Alias("<", ":session stack pop")

	s.mm.registerBuiltinCommands()
}
