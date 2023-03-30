package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
	"golang.org/x/net/proxy"
	"golang.org/x/term"
)

var red = lipgloss.NewStyle().Foreground(lipgloss.Color("#e61919"))
var blue = lipgloss.NewStyle().Foreground(lipgloss.Color("#2da9d2"))
var green = lipgloss.NewStyle().Foreground(lipgloss.Color("#28bd28"))

func chat() {
	opts := struct {
		ChatGPTOptions
		APIKey            string        `cortana:"--openai-api-key, -, -, set your openai api key"`
		Socks5            string        `cortana:"--socks5, -, , set the socks5 proxy"`
		Timeout           time.Duration `cortana:"--timeout, -, 180s, the timeout duration for a request"`
		Interactive       bool          `cortana:"--interactive, -i, true, chat in interactive mode, deprecated"`
		System            string        `cortana:"--system, -,, the optional system prompt for initializing the chatgpt"`
		Filename          string        `cortana:"--file, -f, ,send the file content after sending the text(if supplied)"`
		Verbose           bool          `cortana:"--verbose, -v, false, print verbose messages"`
		Stdin             bool          `cortana:"--stdin, -, false, read from stdin, works as '-f --'"`
		NonInteractive    bool          `cortana:"--non-interactive, -n, false, chat in none interactive mode"`
		DisableAutoShrink bool          `cortana:"--disable-auto-shrink, -, false, disable auto shrink messages when tokens limit exceeded"`
		SessionDir        string        `cortana:"--session-dir, -, ~/.guru/session, the session directory"`
		SessionID         string        `cortana:"--session-id, -s,, the session id"`
		Text              string
	}{}
	cortana.Parse(&opts)
	opts.Interactive = !opts.NonInteractive
	if !opts.Stdin {
		opts.Stdin = opts.Filename == "--"
	}

	verbose := func(s string) {
		if opts.Verbose {
			fmt.Println(s)
		}
	}

	// create the session directory if necessary
	if opts.SessionDir == "" {
		opts.SessionDir = "./"
	}
	if opts.SessionDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		opts.SessionDir = path.Join(home, opts.SessionDir[1:])
	}

	if _, err := os.Stat(opts.SessionDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(opts.SessionDir, 0755); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
	sess := newSession(opts.SessionDir)
	sess.registerCommands()
	if err := sess.open(opts.SessionID); err != nil {
		log.Fatal(err)
	}
	defer sess.close()
	// only listen on command events after open(to avoid being fired by replaying)
	sess.listenOnBuiltins()

	ctx := context.Background()
	cli := &http.Client{Timeout: opts.Timeout}
	if opts.Socks5 != "" {
		verbose(blue.Render(fmt.Sprintf("using socks5 proxy: %s", opts.Socks5)))
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
	c := &ChatGPTClient{
		cli:  cli,
		opts: opts.ChatGPTOptions,
	}
	c.RegisterBuiltinCommands()

	if opts.System != "" {
		sess.append(&Message{Role: System, Content: opts.System})
	}
	if opts.Text != "" {
		sess.append(&Message{Role: User, Content: opts.Text})
	}

	var content []byte
	var err error
	if opts.Stdin {
		verbose(blue.Render("read from stdin"))
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal("read stdin failed")
		}
	} else if opts.Filename != "" {
		if strings.HasPrefix(opts.Filename, "http") {
			verbose(blue.Render("fetch url: " + opts.Filename))
			resp, err := http.Get(opts.Filename)
			if err != nil {
				log.Fatal("http get failed", err)
			}
			verbose(blue.Render("HTTP Code: " + resp.Status))
			content, err = io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal("read http body failed", err)
			}
			resp.Body.Close()
		} else if opts.Filename != "" {
			verbose(blue.Render("read local file: " + opts.Filename))
			content, err = os.ReadFile(opts.Filename)
			if err != nil {
				log.Fatal("read file failed", err)
			}
		}
	}
	if len(content) > 0 {
		sess.append(&Message{Role: User, Content: string(content)})
	}

	ask := func() error {
		verbose(blue.Render(fmt.Sprintf("send messages: %d", len(sess.messages()))))
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		if opts.ChatGPTOptions.Stream {
		retry:
			s, err := tui.Display[tui.Model[chan *AnswerChunk], chan *AnswerChunk](ctx,
				tui.NewSpinnerModel("", func() (chan *AnswerChunk, error) {
					return c.stream(ctx, opts.APIKey, sess.messages())
				}))
			if err != nil {
				return err
			}
			// ctrl+c interrupted
			if s == nil {
				return nil
			}
			content, err := tui.Display[tui.Model[string], string](ctx, tui.NewStreamModel(s, func(event *AnswerChunk) (string, error) {
				if event.Error.Message != "" {
					return "", fmt.Errorf("%s: %s", event.Error.Code, event.Error.Message)
				}
				if len(event.Choices) == 0 {
					return "", nil
				}
				return event.Choices[0].Delta.Content, nil
			}))
			if err != nil {
				if strings.Contains(err.Error(), "context_length_exceeded") && len(sess.messages()) > 1 {
					if !opts.DisableAutoShrink {
						n := sess.mm.autoShrink()
						if n > 0 {
							fmt.Println(blue.Render(fmt.Sprintf(
								"%d message%s shrinked because of tokens limitation", n,
								func() string {
									if n > 1 {
										return "s"
									}
									return ""
								}())))
							goto retry
						}
					} else {
						err = fmt.Errorf("%w\n\nUse `:messages shrink <expr>` to reduce the tokens", err)
					}
				}
				return err
			}
			sess.append(&Message{Role: Assistant, Content: content})
			return nil
		}
		var ans *Answer
		var err error
		if term.IsTerminal(int(os.Stdout.Fd())) {
			ans, err = tui.Display[tui.Model[*Answer], *Answer](ctx,
				tui.NewSpinnerModel("waiting response...", func() (*Answer, error) {
					return c.ask(ctx, opts.APIKey, sess.messages())
				}))
		} else {
			ans, err = c.ask(ctx, opts.APIKey, sess.messages())
		}
		if err != nil {
			return err
		}

		// maybe ctrl+c interrupted
		if ans == nil {
			return nil
		}

		verbose(blue.Render(fmt.Sprintf("%#v", ans)))
		if ans.Error.Message != "" {
			return fmt.Errorf(ans.Error.Message)
		}

		verbose(blue.Render("render with markdown"))
		out := bytes.NewBuffer(nil)
		for _, choice := range ans.Choices {
			content := strings.TrimSpace(choice.Message.Content)
			out.WriteByte('\n')
			out.WriteString(content)
			out.WriteByte('\n')

			sess.append(choice.Message)
		}
		tui.Display[tui.Model[string], string](ctx, tui.NewMarkdownModel(out.String()))

		if opts.Interactive {
			fmt.Println(green.Render(fmt.Sprintf("Cost : prompt(%d) completion(%d) total(%d)",
				ans.Usage.PromptTokens, ans.Usage.CompletionTokens, ans.Usage.TotalTokens)))
		}
		return nil
	}

	if opts.Text != "" || opts.System != "" || opts.Stdin || opts.Filename != "" {
		if err := ask(); err != nil {
			fmt.Println(red.Render(err.Error()))
		}
	}

	if !opts.Interactive {
		return
	}

	talk := func(text string) {
		var err error
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}

		// run a shell command
		if text[0] == '$' {
			out, err := runCommand(text[1:])
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(out)
			sess.append(&Message{Role: User, Content: out})
			return
		}
		// run a builtin command
		if text[0] == ':' || text[0] == '<' || text[0] == '>' {
			args := strings.FieldsFunc(text, func() func(r rune) bool {
				arounded := false
				return func(r rune) bool {
					if r == '\'' || r == '"' {
						arounded = !arounded
						return true
					}
					if unicode.IsSpace(r) && !arounded {
						return true
					}
					return false
				}
			}())
			text = strings.TrimSpace(builtins.Launch(ctx, args))
			if text == "" {
				return
			}
		}

		// avoid adding a dupicated input text when an error occurred for the
		// last text
		if l := len(sess.messages()); l > 0 {
			last := sess.messages()[l-1].Content
			if err == nil || last != text {
				sess.append(&Message{Role: User, Content: text})
			}
		} else {
			sess.append(&Message{Role: User, Content: text})
		}

		err = ask()
		if err != nil {
			fmt.Println(red.Render(err.Error()))
		}
	}

	repl(&livePrompt{prompt: "ChatGPT >", style: green, append: ">"}, talk)

}

func main() {
	unmarshaler := cortana.UnmarshalFunc(json.Unmarshal)
	cortana.AddConfig("guru.json", unmarshaler)
	cortana.AddConfig("~/.config/guru/guru.json", unmarshaler)
	cortana.Use(cortana.ConfFlag("--conf", "-c", unmarshaler))

	cortana.AddCommand("chat", chat, "chat with ChatGPT")
	cortana.AddCommand("serve", serve, "serve as an ssh app")

	cortana.Alias("review", `chat --system 帮我Review以下代码,并给出优化意见,用Markdown渲染你的回应`)
	cortana.Alias("translate", `chat --system 帮我翻译以下文本到中文,用Markdown渲染你的回应`)
	cortana.Alias("unittest", `chat --system 为我指定的函数编写一个单元测试,用Markdown渲染你的回应`)
	cortana.Alias("commit message", `chat --system "give me a one line commit message based on the diff with less than 15 words"`)
	cortana.Launch()
}
