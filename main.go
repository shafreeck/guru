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
func marshalJSON(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return pretty.Color(pretty.Pretty(data), nil), nil
	}
	return data, nil
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
		Socks5 string `cortana:"--socks5"`
		APIKey string `cortana:"--openai-api-key, -, -"`
		Text   string
	}{}
	cortana.Parse(&opts)
	dailer, err := proxy.SOCKS5("tcp", opts.Socks5, nil, proxy.Direct)
	if err != nil {
		log.Fatal(err)
	}
	cli := &http.Client{Timeout: 30 * time.Second}
	if opts.Socks5 != "" {
		cli.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dailer.Dial(network, addr)
			},
		}
	}

	if opts.Text != "" {
		ans, err := ask(cli, opts.APIKey, []*Message{{Role: User, Content: opts.Text}})
		if err != nil {
			log.Fatal(err)
		}
		printJSON(ans)
		return
	}

	oldState, err := term.MakeRaw(0)
	if err != nil {
		panic(err)
	}
	defer term.Restore(0, oldState)

	rw := struct {
		io.Reader
		io.Writer
	}{Reader: os.Stdin, Writer: os.Stdout}
	term := term.NewTerminal(rw, "ChatGPT > ")

	writeErr := func(err error) {
		term.Write([]byte(err.Error()))
		term.Write([]byte("\n"))
	}

	var messages []*Message
	for {
		text, err := term.ReadLine()
		if err != nil {
			writeErr(err)
			break
		}
		if text == "" {
			continue
		}
		messages = append(messages, &Message{Role: User, Content: text})

		ans, err := ask(cli, opts.APIKey, messages)
		if err != nil {
			writeErr(err)
			continue
		}
		if ans.Error.Message != "" {
			term.Write([]byte(ans.Error.Message))
			continue
		}
		for _, choice := range ans.Choices {
			term.Write([]byte(strings.TrimSpace(choice.Message.Content)))
			term.Write([]byte("\n"))
			messages = append(messages, choice.Message)
		}
	}
}

func main() {
	unmarshaler := cortana.UnmarshalFunc(json.Unmarshal)
	cortana.AddConfig("guru.json", unmarshaler)
	cortana.Use(cortana.ConfFlag("--conf", "-c", unmarshaler))

	cortana.AddCommand("chat", chat, "chat with ChatGPT")
	cortana.Launch()
}
