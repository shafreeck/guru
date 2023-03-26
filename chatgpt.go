package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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
	Stream           bool    `json:"stream,omitempty" cortana:"--chatgpt.stream, -s, true, If set, partial message deltas will be sent, like in ChatGPT. Tokens will be sent as data-only server-sent events as they become available, with the stream terminated by a data: [DONE] message. See the OpenAI Cookbook for example code."`
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
