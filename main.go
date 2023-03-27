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
	"strings"
	"time"
	"unicode"

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
	"golang.org/x/net/proxy"
	"golang.org/x/term"
)

func completer(d prompt.Document) []prompt.Suggest {
	if d.LastKeyStroke() != prompt.Tab {
		return nil
	}

	line := strings.TrimLeft(d.CurrentLineBeforeCursor(), " ")
	if line == "" {
		return nil
	}
	switch line[0] {
	case '$':
		return cmdCompleter(d)
	case ':':
		return builtinCompleter(d)
	}
	return nil
}

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
		Text              string
	}{}
	cortana.Parse(&opts)
	opts.Interactive = !opts.NonInteractive

	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#e61919"))
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("#2da9d2"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#28bd28"))

	verbose := func(s string) {
		if opts.Verbose {
			fmt.Println(s)
		}
	}

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

	var messages []*Message
	mm := &messageManager{}
	// Register messageManager commands
	builtins.AddCommand(":messages", func() {
		mm.display(messages)
	}, "list messages")
	builtins.AddCommand(":messages delete", func() {
		messages = mm.delete(messages)
	}, "delete messages")
	builtins.AddCommand(":messages shrink", func() {
		messages = mm.shrink(messages)
	}, "shrink messages")
	builtins.AddCommand(":messages show", func() {
		mm.show(messages)
	}, "show certain messages")
	builtins.Alias(":messages reset", ":messages shrink 0:0")
	builtins.Alias(":reset", ":messages reset")

	if opts.System != "" {
		messages = append(messages, &Message{Role: System, Content: opts.System})
	}
	if opts.Text != "" {
		messages = append(messages, &Message{Role: User, Content: opts.Text})
	}
	if opts.Filename != "" || opts.Stdin {
		var content []byte
		var err error
		if opts.Filename == "--" || opts.Stdin {
			verbose(blue.Render("read from stdin"))
			content, err = io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatal("read stdin failed")
			}
		} else if strings.HasPrefix(opts.Filename, "http") {
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
		messages = append(messages, &Message{Role: User, Content: string(content)})
	}

	ask := func() error {
		verbose(blue.Render(fmt.Sprintf("send messages: %d", len(messages))))
		if opts.ChatGPTOptions.Stream {
		retry:
			s, err := tui.Display[tui.Model[chan *AnswerChunk], chan *AnswerChunk](ctx,
				tui.NewSpinnerModel("", func() (chan *AnswerChunk, error) {
					return c.stream(ctx, opts.APIKey, messages)
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
				if strings.Contains(err.Error(), "context_length_exceeded") && len(messages) > 1 {
					if !opts.DisableAutoShrink {
						var n int
						n, messages = mm.autoShrink(messages)
						if n > 0 {
							if n == 1 {
								fmt.Println(blue.Render(fmt.Sprintf("%d message shrinked because of tokens limitation", n)))
							} else {
								fmt.Println(blue.Render(fmt.Sprintf("%d messages shrinked because of tokens limitation", n)))
							}
						}
						goto retry
					}
					err = fmt.Errorf("%w\n\nUse `:messages shrink <expr>` to reduce the tokens", err)
				}
				return err
			}
			messages = append(messages, &Message{Role: Assistant, Content: content})
			return nil
		}
		var ans *Answer
		var err error
		if term.IsTerminal(int(os.Stdout.Fd())) {
			ans, err = tui.Display[tui.Model[*Answer], *Answer](ctx,
				tui.NewSpinnerModel("waiting response...", func() (*Answer, error) {
					return c.ask(ctx, opts.APIKey, messages)
				}))
		} else {
			ans, err = c.ask(ctx, opts.APIKey, messages)
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

			messages = append(messages, choice.Message)
		}
		tui.Display[tui.Model[string], string](ctx, tui.NewMarkdownModel(out.String()))

		if opts.Interactive {
			fmt.Println(green.Render(fmt.Sprintf("Cost : prompt(%d) completion(%d) total(%d)",
				ans.Usage.PromptTokens, ans.Usage.CompletionTokens, ans.Usage.TotalTokens)))
		}
		return nil
	}

	if opts.Text == "" && opts.Filename == "" {
		opts.Interactive = true
	}

	if len(messages) > 0 {
		if err := ask(); err != nil {
			fmt.Println(red.Render(err.Error()))
		}
		if !opts.Interactive {
			return
		}
	}

	talk := func(text string) {
		// update to ChatGPT client
		// TODO: use a more proper way
		defer func() {
			c.messages = messages
		}()

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
			messages = append(messages, &Message{Role: User, Content: out})
			return
		}
		// run a builtin command
		if text[0] == ':' {
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
			text = strings.TrimSpace(builtins.Launch(ctx, args...))
			if text == "" {
				return
			}
		}

		// avoid adding a dupicated input text when an error occurred for the
		// last text
		if l := len(messages); l > 0 {
			last := messages[l-1].Content
			if err == nil || last != text {
				messages = append(messages, &Message{Role: User, Content: text})
			}
		} else {
			messages = append(messages, &Message{Role: User, Content: text})
		}

		err = ask()
		if err != nil {
			fmt.Println(red.Render(err.Error()))
		}
	}

	prompt.New(talk, completer,
		prompt.OptionPrefix("ChatGPT > "),
		prompt.OptionPrefixTextColor(prompt.Green),
	).Run()
}

func main() {
	unmarshaler := cortana.UnmarshalFunc(json.Unmarshal)
	cortana.AddConfig("guru.json", unmarshaler)
	cortana.AddConfig("~/.config/guru/guru.json", unmarshaler)
	cortana.Use(cortana.ConfFlag("--conf", "-c", unmarshaler))

	cortana.AddCommand("chat", chat, "chat with ChatGPT")
	cortana.Alias("review", `chat --system 帮我Review以下代码,并给出优化意见,用Markdown渲染你的回应`)
	cortana.Alias("translate", `chat --system 帮我翻译以下文本到中文,用Markdown渲染你的回应`)
	cortana.Alias("unittest", `chat --system 为我指定的函数编写一个单元测试,用Markdown渲染你的回应`)
	cortana.Alias("commit message", `chat --system "give me a one line commit message based on the diff with less than 15 words"`)
	cortana.Launch()
}
