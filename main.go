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

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/shafreeck/cortana"
	"github.com/tidwall/pretty"
	"golang.org/x/net/proxy"
	"golang.org/x/term"
)

func printJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println(string(
			pretty.Color(pretty.Pretty(data), nil)))
	} else {
		fmt.Println(string(data))
	}
}

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
	Model    string     `json:"model"`
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

func ask(cli *http.Client, apiKey string, messages []*Message) (*Answer, error) {
	url := "https://api.openai.com/v1/chat/completions"

	question := Question{
		Model:    "gpt-3.5-turbo",
		Messages: messages,
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

	resp, err := cli.Do(req)
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

func chat() {
	opts := struct {
		Socks5      string        `cortana:"--socks5, -, , set the socks5 proxy"`
		APIKey      string        `cortana:"--openai-api-key, -, -, set your openai api key"`
		Timeout     time.Duration `cortana:"--timeout, -, 30s, the timeout duration for a request"`
		Interactive bool          `cortana:"--interactive, -i, false, chat in interactive mode, it will be in this mode default if no text supplied"`
		System      string        `cortana:"--system, -, , the optional system prompt for initializing the chatgpt"`
		Text        string
	}{}
	cortana.Parse(&opts)
	cli := &http.Client{Timeout: opts.Timeout}
	if opts.Socks5 != "" {
		blue := lipgloss.NewStyle().Foreground(lipgloss.Color("#2da9d2"))

		fmt.Println(blue.Render(fmt.Sprintf("using socks5 proxy: %s", opts.Socks5)))
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

	if opts.Text != "" && !opts.Interactive {
		ans, err := ask(cli, opts.APIKey, []*Message{{Role: User, Content: opts.Text}})
		if err != nil {
			log.Fatal(err)
		}
		printJSON(ans)
		return
	}

	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#e61919"))

	var messages []*Message
	if opts.System != "" {
		messages = append(messages, &Message{Role: System, Content: opts.System})
	}
	if opts.Text != "" {
		messages = append(messages, &Message{Role: User, Content: opts.Text})
	}

	// use the markdown render to render the response
	mdr, err := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithAutoStyle(),
	)
	if err != nil {
		log.Fatal(err)
	}
	ask := func(messages []*Message) error {
		ans, err := ask(cli, opts.APIKey, messages)
		if err != nil {
			return err
		}
		if ans.Error.Message != "" {
			return fmt.Errorf(ans.Error.Message)
		}
		for _, choice := range ans.Choices {
			fmt.Println()
			content := strings.TrimSpace(choice.Message.Content)
			out, err := mdr.Render(content)
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Println(out)
			messages = append(messages, choice.Message)
		}
		return nil
	}
	if len(messages) > 0 {
		ask(messages)
	}

	talk := func(text string) {
		var err error
		if text == "" {
			return
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

	prompt.New(talk, func(d prompt.Document) []prompt.Suggest { return nil },
		prompt.OptionPrefix("ChatGPT > "),
		prompt.OptionPrefixTextColor(prompt.Green),
	).Run()
}

func main() {
	unmarshaler := cortana.UnmarshalFunc(json.Unmarshal)
	cortana.AddConfig("guru.json", unmarshaler)
	cortana.Use(cortana.ConfFlag("--conf", "-c", unmarshaler))

	cortana.AddCommand("chat", chat, "chat with ChatGPT")
	cortana.Launch()
}
