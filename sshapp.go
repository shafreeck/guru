package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/logging"
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
	s, err := wish.NewServer(wish.WithAddress(g.address),
		wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
			// no auth
			if g.auth == "" {
				return true
			}
			return password == g.auth
		}),
		wish.WithMiddleware(activeterm.Middleware(),
			func(h ssh.Handler) ssh.Handler {
				return g.handle
			}, logging.Middleware()))

	if err != nil {
		log.Fatal(err)
	}
	g.s = s
	return s.ListenAndServe()
}

func (g *guruSSHServer) handle(sess ssh.Session) {
	tui.Stdin = sess
	tui.Stdout = sess
	tui.Stderr = sess
	builtins.Use(cortana.WithStdout(sess))
	builtins.Use(cortana.WithStderr(sess))
	cortana.Use(cortana.WithStdout(sess))
	cortana.Use(cortana.WithStderr(sess))

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
