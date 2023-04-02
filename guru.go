package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/shafreeck/cortana"
	"golang.org/x/net/proxy"
)

// Guru is the enter of command line
type Guru struct {
	textStyle      lipgloss.Style
	errStyle       lipgloss.Style
	promptStyle    lipgloss.Style
	highlightStyle lipgloss.Style

	isVerbose bool

	// the input/output
	stdin  io.ReadCloser
	stdout io.Writer
	stderr io.Writer
}

type GuruOption func(g *Guru)

func WithStdin(stdin io.ReadCloser) GuruOption {
	return func(g *Guru) {
		g.stdin = stdin
	}
}
func WithStdout(stdout io.Writer) GuruOption {
	return func(g *Guru) {
		g.stdout = stdout
	}
}
func WithStderr(stderr io.Writer) GuruOption {
	return func(g *Guru) {
		g.stderr = stderr
	}
}

func New() *Guru {
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e61919"))       //red
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#79b3ec"))      //blue
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0aacf8")) //blue
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#13f911"))    //green

	return &Guru{
		errStyle:       errStyle,
		textStyle:      textStyle,
		promptStyle:    promptStyle,
		highlightStyle: highlightStyle,
		stdin:          os.Stdin,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
	}
}

type ChatCommandOptions struct {
	ChatGPTOptions
	APIKey            string        `cortana:"--openai-api-key, -, -, set your openai api key"`
	Socks5            string        `cortana:"--socks5, -, , set the socks5 proxy"`
	Timeout           time.Duration `cortana:"--timeout, -, 180s, the timeout duration for a request"`
	System            string        `cortana:"--system, -,, the optional system prompt for initializing the chatgpt"`
	Filename          string        `cortana:"--file, -f, ,send the file content after sending the text(if supplied)"`
	Verbose           bool          `cortana:"--verbose, -v, false, print verbose messages"`
	Stdin             bool          `cortana:"--stdin, -, false, read from stdin, works as '-f --'"`
	NonInteractive    bool          `cortana:"--non-interactive, -n, false, chat in none interactive mode"`
	DisableAutoShrink bool          `cortana:"--disable-auto-shrink, -, false, disable auto shrink messages when tokens limit exceeded"`
	Dir               string        `cortana:"--dir,-, ~/.guru, the guru directory"`
	SessionID         string        `cortana:"--session-id, -s,, the session id"`
	Text              string
}

// chatCommand chats with ChatGPT
func (g *Guru) ChatCommand() {
	opts := &ChatCommandOptions{}
	cortana.Parse(opts)

	// create directories if necessary
	opts.Dir = expandPath(opts.Dir)
	if err := initGuruDirs(opts.Dir); err != nil {
		g.Fatalln("initialize guru directories failed", err)
	}

	// create session
	sessionDir := path.Join(opts.Dir, "session")
	sess := NewSession(sessionDir, WithCommandOutput(g), WithHighlightStyle(g.highlightStyle))
	if err := sess.Open(opts.SessionID); err != nil {
		g.Fatalln(err)
	}
	defer sess.Close()

	httpCli := g.getHTTPClient(opts)

	// load awesome prompts
	promptDir := path.Join(opts.Dir, "prompt")
	ap := NewAwesomePrompts(promptDir, httpCli, g)
	if err := ap.Load(); err != nil {
		g.Fatalln(err)
	}

	// read from stdin or file
	var err error
	var content string
	if !opts.Stdin {
		opts.Stdin = opts.Filename == "--"
	}
	if opts.Stdin {
		opts.NonInteractive = true
		content, err = g.readStdin()
	} else if opts.Filename != "" && opts.Filename != "--" {
		content, err = g.readFile(opts.Filename)
	}
	if err != nil {
		g.Fatalln(err)
	}
	if content != "" {
		sess.Append(&Message{Role: User, Content: content})
	}

	// new a ChatGPT client and run the command
	cc := NewChatCommand(sess, ap, httpCli, opts)

	// enter the REPL routine
	lp := &LivePrompt{
		Prefix:         "guru",
		Delimiter:      ">",
		PrefixStyle:    g.promptStyle,
		SuffixStyle:    g.promptStyle,
		DelimiterStyle: g.promptStyle,
	}
	eval := func(text string) {
		// handle sys or builtin commands
		if len(text) > 0 {
			switch c := text[0]; c {
			case '>', '<':
				if c == '>' {
					lp.PushSuffix(">")
				} else if c == '<' {
					lp.PopSuffix()
				}
				fallthrough
			case ':':
				if cont := builtinCommandEval(sess, text); !cont {
					return
				}
				text = ""
			case '$':
				if cont := sysCommandEval(sess, text[1:]); !cont {
					return
				}
				text = ""
			}
		}

		copts := &ChatOptions{
			ChatGPTOptions:    opts.ChatGPTOptions,
			Text:              text,
			System:            opts.System,
			Verbose:           opts.Verbose,
			NonInteractive:    opts.NonInteractive,
			DisableAutoShrink: opts.DisableAutoShrink,
		}
		cc.Talk(copts)
	}

	// Evaluate first before entering interactive mode
	eval(opts.Text)

	if opts.NonInteractive {
		return
	}

	repl := NewRepl(lp)
	if err := repl.Loop(NewEvaluator(sess, lp, eval)); err != nil {
		g.Fatalln(err)
	}
}

func (g *Guru) ServeSSH() {
	opts := struct {
		Address string `cortana:"address, -, :2023"`
		Auth    string `cortana:"--auth, -, ,the auth password"`
	}{}
	cortana.Parse(&opts)

	gs := newGuruSSHServer(opts.Address, opts.Auth)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := gs.serve(); err != nil {
			log.Fatal(err)
		}
	}()

	fmt.Println("serving on:", opts.Address)
	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := gs.s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (g *Guru) readStdin() (string, error) {
	data, err := io.ReadAll(g.stdin)
	if err != nil {
		return "", nil
	}
	return string(data), nil
}

func (g *Guru) readFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", nil
	}
	return string(data), err
}

func (g *Guru) StylePrint(style lipgloss.Style, a ...any) {
	var ss []any
	for _, s := range a {
		ss = append(ss, style.Render(fmt.Sprint(s)))
	}
	fmt.Fprint(g.stdout, ss...)
}
func (g *Guru) StylePrintf(style lipgloss.Style, format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	rendered := style.Render(s)
	fmt.Fprint(g.stdout, rendered)
}

func (g *Guru) StylePrintln(style lipgloss.Style, a ...any) {
	g.StylePrint(style, a...)
	fmt.Fprintln(g.stdout)
}

func (g *Guru) Error(a ...any) {
	g.StylePrint(g.errStyle, a...)
}

func (g *Guru) Errorf(format string, a ...any) {
	g.StylePrintf(g.errStyle, format, a...)
}

func (g *Guru) Errorln(a ...any) {
	g.StylePrintln(g.errStyle, a...)
}

func (g *Guru) Print(a ...any) {
	g.StylePrint(g.textStyle, a...)
}

func (g *Guru) Printf(format string, a ...any) {
	g.StylePrintf(g.textStyle, format, a...)
}

func (g *Guru) Println(a ...any) {
	g.StylePrintln(g.textStyle, a...)
}

func (g *Guru) Fatalln(a ...any) {
	g.Println(a...)
	os.Exit(-1)
}

// initGuruDirs creates directories guru needed
// it returns nil if the directories are exist
func initGuruDirs(dir string) error {
	sessionDir := path.Join(dir, "session")
	promptDir := path.Join(dir, "prompt")

	for _, d := range []string{dir, sessionDir, promptDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

// expandPath expands ~ or env vars in p
func expandPath(p string) string {
	if p == "" {
		return p
	}
	if p[0] == '~' {
		home, _ := os.UserHomeDir()
		p = path.Join(home, p[1:])
	}
	return os.ExpandEnv(p)
}

func (g *Guru) getHTTPClient(opts *ChatCommandOptions) *http.Client {
	cli := &http.Client{Timeout: opts.Timeout}
	if opts.Socks5 != "" {
		g.verbose(fmt.Sprintf("using socks5 proxy: %s", opts.Socks5))
		dailer, err := proxy.SOCKS5("tcp", opts.Socks5, nil, proxy.Direct)
		if err != nil {
			log.Fatal(err)
		}

		cli.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dailer.Dial(network, addr)
			},
		}
	}
	return cli
}
func (g *Guru) verbose(text string) {
	if g.isVerbose {
		g.Println(text)
	}
}