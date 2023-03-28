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
	"github.com/charmbracelet/wish/logging"
	"github.com/shafreeck/cortana"
)

func serve() {
	opts := struct {
		Address string `cortana:"address, -, :2023"`
	}{}
	cortana.Parse(&opts)

	g := newGuruSSHServer(opts.Address)

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
	address string
}

func newGuruSSHServer(address string) *guruSSHServer {
	return &guruSSHServer{cmd: cortana.New(), address: address}
}

func (g *guruSSHServer) serve() error {
	s, err := wish.NewServer(wish.WithAddress(g.address), wish.WithMiddleware(func(h ssh.Handler) ssh.Handler {
		return g.handle
	}, logging.Middleware()))

	if err != nil {
		log.Fatal(err)
	}
	g.s = s
	return s.ListenAndServe()
}

func (g *guruSSHServer) handle(sess ssh.Session) {
	cortana.Launch(sess.Command()...)
}
