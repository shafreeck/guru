package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

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
	w       io.ReadWriteCloser
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

type session struct {
	mm        messageManager
	dir       string
	sid       string
	stack     []string
	stackOnce sync.Once
	history   history
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

func (s *session) remove(sid string) error {
	return os.Remove(path.Join(s.dir, sid))
}

func (s *session) close() {
	// nothing saved, delete the session
	if len(s.history.records) == 0 {
		s.remove(s.sid)
	}
	if s.history.w != nil {
		s.history.w.Close()
	}
}

func (s *session) append(m *Message) {
	s.mm.append(m)
	if err := s.history.append(":append", m); err != nil {
		fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
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
	if len(records) > 0 {
		s.history.records = records
		s.replay(records)
	}
	return nil
}

func (s *session) replay(records []*record) {
	render := &tui.JSONRenderer{}
	fmt.Fprintln(tui.Stdout, blue.Render("replay session:", s.sid))
	for _, r := range records {
		args := strings.Fields(r.Op)
		fmt.Fprint(tui.Stdout, blue.Render(r.Op), " ")
		if r.Msg != nil {
			data, _ := json.Marshal(r.Msg)
			data, _ = render.Render(data)
			fmt.Fprintln(tui.Stdout, string(data))
			args = append(args, "--role", string(r.Msg.Role), r.Msg.Content)
		} else {
			fmt.Fprintln(tui.Stdout)
		}
		builtins.Launch(context.Background(), args)
	}
}

func (s *session) list() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
	}
	for i, entry := range entries {
		if entry.Name() == s.sid {
			fmt.Fprint(tui.Stdout, blue.Render("  *  "))
		} else {
			fmt.Fprintf(tui.Stdout, "%3d. ", i)
		}
		fmt.Fprintln(tui.Stdout, entry.Name())
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

func (s *session) switchSession(sid string) {
	s.close()
	builtins.sess = nil
	s.mm.messages = nil // clear the messages
	s.history = history{}
	s.open(sid)
	builtins.sess = s
}

func (s *session) switchCommand() {
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
		fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
		return
	}

	s.switchSession(opts.SID)
}

func (s *session) removeCommand() {
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
	if err := s.remove(opts.SID); err != nil {
		fmt.Fprintln(tui.Stdout, red.Render(err.Error()))
	}
}

func (s *session) new(ctx context.Context) string {
	opts := struct {
		SID   string   `cortana:"--session-id, -s, ,the session id"`
		Texts []string `cortana:"text"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		return ""
	}

	if opts.SID != "" {
		if _, err := os.Stat(path.Join(s.dir, opts.SID)); err == nil {
			fmt.Fprintln(tui.Stdout, red.Render("session \""+opts.SID+"\" exist"))
			return ""
		}
	}

	s.switchSession(opts.SID)
	fmt.Fprintln(tui.Stdout, blue.Render("session "+s.sid+" created"))

	return strings.Join(opts.Texts, " ")
}

func (s *session) shrink() {
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
		fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
	}
	for _, entry := range entries {
		ids = append(ids, entry.Name())
	}

	size := len(ids)

	parts := strings.Split(opts.Expr, ":")

	if v := parts[0]; v != "" {
		begin, err = strconv.Atoi(parts[0])
		if err != nil {
			fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
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
				fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
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
		s.remove(sid)
		fmt.Fprintln(tui.Stdout, blue.Render(sid+" removed"))
	}
}

func (s *session) historyCommand() {
	render := tui.JSONRenderer{}
	for i, record := range s.history.records {
		data, err := json.Marshal(record)
		if err != nil {
			fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
		}
		text, err := render.Render(data)
		if err != nil {
			fmt.Fprintln(tui.Stderr, red.Render(err.Error()))
		}
		fmt.Fprintf(tui.Stdout, "%3d. %s\n", i, string(text))
	}
}
func (s *session) stackPush(ctx context.Context) string {
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
	fmt.Fprintln(tui.Stdout, blue.Render("step in session: "+s.sid))

	return strings.Join(opts.Texts, " ")
}
func (s *session) stackPop() {
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
}
func (s *session) stackShow() {
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
		fmt.Fprint(out, blue.Render(fmt.Sprintf(" > %s", sid)))
	}
	fmt.Fprintln(tui.Stdout, out.String())
}

func (s *session) registerCommands() {
	builtins.AddCommand(":session new", builtin(s.new), "create a new session")
	builtins.AddCommand(":session remove", s.removeCommand, "delete a session")
	builtins.AddCommand(":session shrink", s.shrink, "shrink sessions")
	builtins.AddCommand(":session list", s.list, "list sessions")
	builtins.AddCommand(":session switch", s.switchCommand, "switch a session")
	builtins.AddCommand(":session history", s.historyCommand, "print history of current session")
	builtins.AddCommand(":session stack", s.stackShow, "show the session stack")
	builtins.AddCommand(":session stack push", builtin(s.stackPush), "create a new session, and stash the current")
	builtins.AddCommand(":session stack pop", s.stackPop, "pop out current session")

	builtins.Alias(":session clear", ":session shrink 0:0")
	builtins.Alias(":stack", ":session stack")
	builtins.Alias(">", ":session stack push")
	builtins.Alias("<", ":session stack pop")
	s.mm.registerMessageCommands()
}
