package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/logging"
	"github.com/creack/pty"
	"github.com/muesli/termenv"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
)

func serve() {
	opts := struct {
		Address string `cortana:"address, -, :2023"`
		Auth    string `cortana:"--auth, -, ,the auth password"`
	}{}
	cortana.Parse(&opts)

	g := newGuruSSHServer(opts.Address, opts.Auth)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := g.serve(); err != nil {
			log.Fatal(err)
		}
	}()

	fmt.Println("serving on:", opts.Address)
	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := g.s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Fatal(err)
	}
}

type guruSSHServer struct {
	s       *ssh.Server
	cmd     *cortana.Cortana
	auth    string
	address string
}

func newGuruSSHServer(address, auth string) *guruSSHServer {
	return &guruSSHServer{cmd: cortana.New(), address: address, auth: auth}
}

func (g *guruSSHServer) serve() error {
	var opts []ssh.Option
	opts = append(opts, wish.WithAddress(g.address))
	if g.auth != "" {
		opts = append(opts, wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
			// no auth
			if g.auth == "" {
				return true
			}
			return password == g.auth
		}))
	}
	opts = append(opts, wish.WithMiddleware(activeterm.Middleware(),
		func(h ssh.Handler) ssh.Handler {
			return g.handle
		}, logging.Middleware()))

	s, err := wish.NewServer(opts...)
	if err != nil {
		log.Fatal(err)
	}
	g.s = s
	return s.ListenAndServe()
}

type sshOutput struct {
	ssh.Session
	tty *os.File
}

func (s *sshOutput) Write(p []byte) (int, error) {
	return s.Session.Write(p)
}

func (s *sshOutput) Read(p []byte) (int, error) {
	return s.Session.Read(p)
}

func (s *sshOutput) Close() error {
	return s.Session.Close()
}

func (s *sshOutput) Name() string {
	return s.tty.Name()
}

func (s *sshOutput) Fd() uintptr {
	return s.tty.Fd()
}

type sshEnviron struct {
	environ []string
}

func (s *sshEnviron) Getenv(key string) string {
	for _, v := range s.environ {
		if strings.HasPrefix(v, key+"=") {
			return v[len(key)+1:]
		}
	}
	return ""
}

func (s *sshEnviron) Environ() []string {
	return s.environ
}

func outputFromSession(s ssh.Session) *termenv.Output {
	sshPty, _, _ := s.Pty()
	_, tty, err := pty.Open()
	if err != nil {
		panic(err)
	}
	o := &sshOutput{
		Session: s,
		tty:     tty,
	}
	environ := s.Environ()
	environ = append(environ, fmt.Sprintf("TERM=%s", sshPty.Term))
	e := &sshEnviron{
		environ: environ,
	}
	return termenv.NewOutput(o, termenv.WithUnsafe(), termenv.WithEnvironment(e))
}

func (g *guruSSHServer) handle(sess ssh.Session) {
	out := outputFromSession(sess)
	tui.Stdin = sess
	tui.Stdout = out
	tui.Stderr = out
	builtins.Use(cortana.WithStdout(sess))
	builtins.Use(cortana.WithStderr(sess))
	cortana.Use(cortana.WithStdout(sess))
	cortana.Use(cortana.WithStderr(sess))
	lipgloss.SetColorProfile(out.ColorProfile())

	args := sess.Command()
	if len(args) == 0 {
		args = append(args, "chat")
	}
	// filter the serve self
	if args[0] == "serve" {
		fmt.Sprintln(sess, "serve command is not supported in the sshapp mode")
	}
	builtins.AddCommand(":exit", func() {
		sess.Close()
	}, "exit the session")
	cortana.Launch(args...)
}
