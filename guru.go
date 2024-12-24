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
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/chroma/quick"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/chzyer/readline"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
	"golang.org/x/net/proxy"
	"gopkg.in/yaml.v3"
)

// Guru is the enter of command line
type Guru struct {
	// the output styles
	textStyle      lipgloss.Style
	errStyle       lipgloss.Style
	promptStyle    lipgloss.Style
	highlightStyle lipgloss.Style

	isVerbose bool
	lp        *LivePrompt
	sess      *Session

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

func New(opts ...GuruOption) *Guru {
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e61919"))       //red
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#79b3ec"))      //blue
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0aacf8")) //blue
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#13f911"))    //green

	g := &Guru{
		errStyle:       errStyle,
		textStyle:      textStyle,
		promptStyle:    promptStyle,
		highlightStyle: highlightStyle,
		stdin:          os.Stdin,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
	}

	// Apply the options
	for _, opt := range opts {
		opt(g)
	}

	return g
}

type ChatCommandOptions struct {
	ChatGPTOptions    `yaml:"chatgpt,omitempty"`
	APIKey            string        `cortana:"--api-key, -, -, set your api key" yaml:"api-key,omitempty"`
	BaseURL           string        `cortana:"--base-url, -, https://api.openai.com/v1, The base URL for the compitable ChatGPT API." yaml:"base-url,omitempty"`
	Socks5            string        `cortana:"--socks5, -, , set the socks5 proxy" yaml:"socks5,omitempty"`
	Timeout           time.Duration `cortana:"--timeout, -, 180s, the timeout duration for a request"  yaml:"timeout,omitempty"`
	System            string        `cortana:"--system, -,, the optional system prompt for initializing the chatgpt" yaml:"system,omitempty"`
	Prompt            string        `cortana:"--prompt, -p, , the prompt to use" yaml:"prompt,omitempty"`
	Filename          string        `cortana:"--file, -f, ,send the file content after sending the text(if supplied)" yaml:"filename,omitempty"`
	Verbose           bool          `cortana:"--verbose, -v, false, print verbose messages" yaml:"verbose,omitempty"`
	Stdin             bool          `cortana:"--stdin, -, false, read from stdin, works as '-f --'" yaml:"stdin,omitempty"`
	Pin               bool          `cortana:"--pin, -, false, pin the initial messages" yaml:"pin,omitempty"`
	Last              bool          `cortana:"--last, -, false, continue the last session" yaml:"-"`
	Executor          string        `cortana:"--executor, -e,, execute what the ai returned using the executor. notice! you should know the risk to enable this flag." yaml:"executor,omitempty"`
	Feedback          bool          `cortana:"--feedback, -, false, feedback the output of executor" yaml:"feedback,omitempty"`
	Oneshot           bool          `cortana:"--oneshot, -1,, avoid maintaining the context, submit the user input and prompt each time" yaml:"oneshot,omitempty"`
	NonInteractive    bool          `cortana:"--non-interactive, -n, false, chat in none interactive mode" yaml:"non-interactive,omitempty"`
	DisableAutoShrink bool          `cortana:"--disable-auto-shrink, -, false, disable auto shrink messages when tokens limit exceeded" yaml:"disable-auto-shrink,omitempty"`
	Dir               string        `cortana:"--dir,-, ~/.guru, the guru directory" yaml:"dir,omitempty"`
	SessionID         string        `cortana:"--session-id, -s,, the session id" yaml:"session-id,omitempty"`
	Renderer          string        `cortana:"--renderer,, markdown, the render type, can be text, markdown, json" yaml:"renderer,omitempty"`
	Texts             []string      `cortana:"text, -" yaml:"-"`
}

// chatCommand chats with ChatGPT
func (g *Guru) ChatCommand() {
	opts := &ChatCommandOptions{}
	cortana.Parse(opts)

	gi := NewGuruInfo(g, opts)
	gi.registerBuiltinCommands()

	// create directories if necessary
	opts.Dir = expandPath(opts.Dir)
	if err := initGuruDirs(opts.Dir); err != nil {
		g.Fatalln("initialize guru directories failed", err)
	}

	// create session
	sessionDir := path.Join(opts.Dir, "session")
	sess := NewSession(sessionDir, WithCommandOutput(g), WithHighlightStyle(g.highlightStyle))
	if opts.SessionID == "" && opts.Last { // open last session
		opts.SessionID = sess.LastSessionID()
	}
	if err := sess.Open(opts.SessionID); err != nil {
		g.Fatalln(err)
	}
	g.sess = sess
	opts.SessionID = sess.sid
	defer sess.Close()

	httpCli := g.getHTTPClient(opts)

	// load awesome prompts
	promptDir := path.Join(opts.Dir, "prompt")
	ap := NewAwesomePrompts(promptDir, httpCli, g)
	if err := ap.Load(); err != nil {
		g.Fatalln(err)
	}

	// add the system and prompt message
	if opts.System != "" {
		sess.Append(&Message{Role: User, Content: opts.System}, opts.Pin)
	}
	if opts.Prompt != "" {
		p := ap.PromptText(opts.Prompt)
		if p == "" {
			g.Errorln("prompt not found:", opts.Prompt)
		}
		pin := opts.Pin
		// pin the prompt message in oneshot mode
		if opts.Oneshot {
			pin = true
		}
		sess.Append(&Message{Role: User, Content: p}, pin)
	}

	// read from stdin or file
	var err error
	var content string
	if !opts.Stdin {
		opts.Stdin = opts.Filename == "--"
	}
	// read from stdin if os.Stdin is not a terminal
	if !readline.IsTerminal(int(os.Stdin.Fd())) {
		opts.Stdin = true
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
		sess.Append(&Message{Role: User, Content: content}, opts.Pin)
	}

	if !readline.IsTerminal(int(os.Stdout.Fd())) {
		opts.NonInteractive = true
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
	g.lp = lp

	eval := func(text string) {
	feedback:
		copts := &ChatOptions{
			ChatGPTOptions:    opts.ChatGPTOptions,
			System:            opts.System,
			Oneshot:           opts.Oneshot,
			Verbose:           opts.Verbose,
			Executor:          opts.Executor,
			Feedback:          opts.Feedback,
			Renderer:          opts.Renderer,
			NonInteractive:    opts.NonInteractive,
			DisableAutoShrink: opts.DisableAutoShrink,
		}
		// add to guru info, so these args could be set by :set command
		gi.copts = copts

		// handle sys or builtin commands
		text, cont := g.handleSysBuiltinCommands(text)
		if !cont { // should not continue
			return
		}
		copts.Text = text

		reply, err := cc.Talk(copts)
		if err != nil {
			g.Errorln(err)
			return
		}

		// handle post talk, the action is executing the reply by far
		if copts.Executor != "" {
			output := g.execute(NewExecutor(opts.Executor), reply)
			g.Println(output)
			if copts.Feedback && output != "" {
				text = output
				goto feedback
			}
		}
	}

	// Evaluate first before entering interactive mode
	if opts.System != "" || len(opts.Texts) != 0 ||
		opts.Stdin || opts.Filename != "" {

		text := strings.Join(opts.Texts, " ")
		sess.Append(&Message{Role: User, Content: text}, opts.Pin)

		// When in oneshot mode, the first talk should supply all
		// the messages from system, stdin, prompts or text.
		// To avoid cleaning the message above, we unset oneshot flag
		// first time, and then restore it before entering the REPL.
		restore := opts.Oneshot
		opts.Oneshot = false
		eval("")
		opts.Oneshot = restore
	}

	if opts.NonInteractive {
		return
	}

	repl := NewRepl(lp)
	if err := repl.Loop(NewEvaluator(sess, lp, eval)); err != nil {
		g.Fatalln(err)
	}
}
func (g *Guru) handleSysBuiltinCommands(text string) (string, bool) {
	if len(text) == 0 {
		return text, true
	}
	switch c := text[0]; c {
	case '>', '<':
		if c == '>' {
			g.lp.PushSuffix(">")
		} else if c == '<' {
			g.lp.PopSuffix()
		}
		fallthrough
	case ':':
		return "", builtinCommandEval(g.sess, text)
	case '$':
		return "", sysCommandEval(g.sess, text[1:])
	}
	return text, true
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
func (g *Guru) ConfigCommand() {
	opts := struct {
		File  string `cortana:"--file, -f, ~/.guru/config, the configuration file"`
		Init  bool   `cortana:"--init, -, false, initialize the configuration file"`
		Key   string `cortana:"key, -"`
		Value string `cortana:"val, -"`
	}{}
	cortana.Parse(&opts)

	opts.File = expandPath(opts.File)

	data, err := os.ReadFile(opts.File)
	if err != nil && !os.IsNotExist(err) {
		g.Fatalln(err)
	}

	// interactive to create the config
	if (opts.Init || os.IsNotExist(err)) &&
		opts.Key == "" && opts.Value == "" {
		// ask for api-key and socks5
		vals, err := tui.Display[tui.Model[[]string], []string](context.Background(),
			tui.NewConfigInputModel("api-key (required)", "socks5 (if have)"))
		if err != nil {
			g.Fatalln(err)
		}
		if vals[0] == "" && vals[1] == "" {
			return
		}
		data, err := yaml.Marshal(ChatCommandOptions{
			APIKey: vals[0],
			Socks5: vals[1],
		})
		if err != nil {
			g.Fatalln(err)
		}
		if err := os.MkdirAll(path.Dir(opts.File), 0755); err != nil {
			g.Fatalln(err)
		}
		if err := os.WriteFile(opts.File, data, 0644); err != nil {
			g.Fatalln(err)
		}
		return
	}

	// show the configrations
	if opts.Key == "" {
		quick.Highlight(g.stdout, string(data), "yaml", "terminal256", "monokai")
		return
	}

	m := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &m); err != nil {
		g.Fatalln(err)
	}
	original := m

	key := opts.Key
	fields := strings.Split(opts.Key, ".")
	if len(fields) == 2 {
		if m[fields[0]] == nil {
			m[fields[0]] = make(map[string]interface{})
		}
		m = m[fields[0]].(map[string]interface{})
		key = fields[1]
	}

	// get the key and return
	if opts.Value == "" {
		fmt.Fprintln(g.stdout, m[key])
		return
	}

	var val interface{}
	val = opts.Value
	if opts.Value == "true" {
		val = true
	} else if opts.Value == "false" {
		val = false
	}
	m[key] = val

	// marshal the original map
	data, err = yaml.Marshal(original)
	if err != nil {
		g.Fatalln(err)
	}
	if err := os.WriteFile(opts.File, data, 0644); err != nil {
		g.Fatalln(err)
	}
}

// execute a command in shell
func (g *Guru) execute(e *Executor, input string) string {
	confirmed, err := tui.Display[tui.Model[bool], bool](context.Background(), tui.NewConfimModel(input))
	if err != nil {
		g.Errorln(err)
	}

	if !confirmed {
		return ""
	}
	out, err := e.Exec(input)
	if err != nil {
		g.Errorln(err)
	}
	return out
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

func (g *Guru) Write(data []byte) (int, error) {
	return g.stdout.Write(data)
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
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dailer.Dial(network, addr)
			},
		}
	} else {
		cli.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		}
	}
	return cli
}
func (g *Guru) verbose(text string) {
	if g.isVerbose {
		g.Println(text)
	}
}
