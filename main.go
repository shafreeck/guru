package main

import (
	"bufio"
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

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
	"github.com/shafreeck/guru/tui"
	"golang.org/x/net/proxy"
	"golang.org/x/term"
)

type ChatRole string

const (
	User      ChatRole = "user"
	System    ChatRole = "system"
	Assistant ChatRole = "assistant"
)

type Message struct {
	Role    ChatRole `json:"role"`
	Content string   `json:"content"`
}

type Question struct {
	ChatGPTOptions
	Messages []*Message `json:"messages"`
}

type Answer struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`

	Choices []struct {
		Message      *Message `json:"message"`
		FinishReason string   `json:"finish_reason"`
		Index        int      `json:"index"`
	} `json:"choices"`

	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}
}
type AnswerChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}
}

type ChatGPTOptions struct {
	Model            string  `json:"model" cortana:"--chatgpt.model, -, gpt-3.5-turbo, ID of the model to use. See the model endpoint compatibility table for details on which models work with the Chat API."`
	Temperature      float32 `json:"temperature" cortana:"--chatgpt.temperature, -, 1, What sampling temperature to use, between 0 and 2. Higher values like 0.8 will make the output more random, while lower values like 0.2 will make it more focused and deterministic."`
	Topp             float32 `json:"top_p" cortana:"--chatgpt.top_p, -, 1, An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top_p probability mass. So 0.1 means only the tokens comprising the top 10% probability mass are considered."`
	N                int     `json:"n" cortana:"--chatgpt.n, -, 1, How many chat completion choices to generate for each input message."`
	Stop             string  `json:"stop,omitempty" cortana:"--chatgpt.stop, -, , Up to 4 sequences where the API will stop generating further tokens."`
	Stream           bool    `json:"stream,omitempty" cortana:"--chatgpt.stream, -s, false, If set, partial message deltas will be sent, like in ChatGPT. Tokens will be sent as data-only server-sent events as they become available, with the stream terminated by a data: [DONE] message. See the OpenAI Cookbook for example code."`
	MaxTokens        int     `json:"max_tokens,omitempty" cortana:"--chatgpt.max_tokens, -, 0, The maximum number of tokens to generate in the chat completion."`
	PresencePenalty  float32 `json:"presence_penalty,omitempty" cortana:"--chatgpt.presence_penalty, -, 0, Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics."`
	FrequencyPenalty float32 `json:"frequency_penalty,omitempty" cortana:"--chatgpt.frequency_penalty, -, 0, Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to repeat the same line verbatim."`
	User             string  `json:"user,omitempty" cortana:"--chatgpt.user, -, , A unique identifier representing your end-user, which can help OpenAI to monitor and detect abuse."`
}

type ChatGPTClient struct {
	opts ChatGPTOptions
	cli  *http.Client
}

func (c *ChatGPTClient) ask(ctx context.Context, apiKey string, messages []*Message) (*Answer, error) {
	url := "https://api.openai.com/v1/chat/completions"

	question := Question{
		ChatGPTOptions: c.opts,
		Messages:       messages,
	}
	data, err := json.Marshal(question)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.cli.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ans := Answer{}
	if err := json.Unmarshal(data, &ans); err != nil {
		return nil, err
	}
	return &ans, nil
}

func (c *ChatGPTClient) stream(ctx context.Context, apiKey string, messages []*Message) (chan *AnswerChunk, error) {
	ch := make(chan *AnswerChunk)
	url := "https://api.openai.com/v1/chat/completions"

	question := Question{
		ChatGPTOptions: c.opts,
		Messages:       messages,
	}
	data, err := json.Marshal(question)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.cli.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			ansc := &AnswerChunk{}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			prefix := "data:"
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			if line == "data: [DONE]" {
				return
			}
			text := line[len(prefix):]

			if err := json.Unmarshal([]byte(text), ansc); err != nil {
				ansc.Error.Message = err.Error()
				ansc.Error.Type = "command_fail"
			}

			select {
			case <-ctx.Done():
				return
			case ch <- ansc:
			}
		}
	}()
	return ch, nil
}

func completer(d prompt.Document) []prompt.Suggest {
	return []prompt.Suggest{
		{Text: "test"},
	}
}

func chat() {
	opts := struct {
		ChatGPTOptions
		APIKey      string        `cortana:"--openai-api-key, -, -, set your openai api key"`
		Socks5      string        `cortana:"--socks5, -, , set the socks5 proxy"`
		Timeout     time.Duration `cortana:"--timeout, -, 180s, the timeout duration for a request"`
		Interactive bool          `cortana:"--interactive, -i, false, chat in interactive mode, it will be in this mode default if no text supplied"`
		System      string        `cortana:"--system, -,, the optional system prompt for initializing the chatgpt"`
		Filename    string        `cortana:"--file, -f, ,send the file content after sending the text(if supplied)"`
		Verbose     bool          `cortana:"--verbose, -v, false, print verbose messages"`
		Text        string
	}{}
	cortana.Parse(&opts)

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

	var messages []*Message
	if opts.System != "" {
		messages = append(messages, &Message{Role: System, Content: opts.System})
	}
	if opts.Text != "" {
		messages = append(messages, &Message{Role: User, Content: opts.Text})
	}
	if opts.Filename != "" {
		var content []byte
		var err error
		if opts.Filename == "--" {
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
		} else {
			verbose(blue.Render("read local file: " + opts.Filename))
			content, err = os.ReadFile(opts.Filename)
			if err != nil {
				log.Fatal("read file failed", err)
			}
		}
		messages = append(messages, &Message{Role: User, Content: string(content)})
	}

	ask := func(messages []*Message) error {
		verbose(blue.Render(fmt.Sprintf("send messages: %d", len(messages))))
		if opts.ChatGPTOptions.Stream {
			s, err := c.stream(ctx, opts.APIKey, messages)
			if err != nil {
				return err
			}
			tui.Display[tui.Model[string], string](ctx, tui.NewStreamModel(s, func(event *AnswerChunk) (string, error) {
				if len(event.Choices) == 0 {
					return "", nil
				}
				return event.Choices[0].Delta.Content, nil
			}))
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
		if err := ask(messages); err != nil {
			fmt.Println(red.Render(err.Error()))
		}
		if !opts.Interactive {
			return
		}
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
			messages = append(messages, &Message{Role: User, Content: out})
			return
		}
		if text == "exit" || text == "quit" {
			os.Exit(0)
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

		err = ask(messages)
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
	cortana.Launch()
}
