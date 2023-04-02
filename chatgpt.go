package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shafreeck/guru/chat"
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

func (q *Question) New() any {
	return &Question{}
}
func (q *Question) Marshal() ([]byte, error) {
	return json.Marshal(q)
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

	Choices []AnswerChoice `json:"choices"`

	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param"`
		Code    string `json:"code"`
	}
}

func (a *Answer) New() any {
	return &Answer{}
}
func (a *Answer) Unmarshal(data []byte) error {
	return json.Unmarshal(data, a)
}

type AnswerChoice struct {
	Message      *Message `json:"message"`
	FinishReason string   `json:"finish_reason"`
	Index        int      `json:"index"`
}

type AnswerError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
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
	Error AnswerError `json:"error"`
}

func (ac *AnswerChunk) New() any {
	return &AnswerChunk{}
}
func (ac *AnswerChunk) Unmarshal(data []byte) error {
	return json.Unmarshal(data, ac)
}
func (ac *AnswerChunk) SetError(err error) {
	ac.Error.Type = "guru_inner_error"
	ac.Error.Message = err.Error()
}

type ChatGPTOptions struct {
	Model            string  `json:"model" cortana:"--chatgpt.model, -, gpt-3.5-turbo, ID of the model to use. See the model endpoint compatibility table for details on which models work with the Chat API."`
	Temperature      float32 `json:"temperature" cortana:"--chatgpt.temperature, -, 1, What sampling temperature to use, between 0 and 2. Higher values like 0.8 will make the output more random, while lower values like 0.2 will make it more focused and deterministic."`
	Topp             float32 `json:"top_p" cortana:"--chatgpt.top_p, -, 1, An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top_p probability mass. So 0.1 means only the tokens comprising the top 10% probability mass are considered."`
	N                int     `json:"n" cortana:"--chatgpt.n, -, 1, How many chat completion choices to generate for each input message."`
	Stop             string  `json:"stop,omitempty" cortana:"--chatgpt.stop, -, , Up to 4 sequences where the API will stop generating further tokens."`
	Stream           bool    `json:"stream,omitempty" cortana:"--chatgpt.stream, -, true, If set, partial message deltas will be sent, like in ChatGPT. Tokens will be sent as data-only server-sent events as they become available, with the stream terminated by a data: [DONE] message. See the OpenAI Cookbook for example code."`
	MaxTokens        int     `json:"max_tokens,omitempty" cortana:"--chatgpt.max_tokens, -, 0, The maximum number of tokens to generate in the chat completion."`
	PresencePenalty  float32 `json:"presence_penalty,omitempty" cortana:"--chatgpt.presence_penalty, -, 0, Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics."`
	FrequencyPenalty float32 `json:"frequency_penalty,omitempty" cortana:"--chatgpt.frequency_penalty, -, 0, Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to repeat the same line verbatim."`
	User             string  `json:"user,omitempty" cortana:"--chatgpt.user, -, , A unique identifier representing your end-user, which can help OpenAI to monitor and detect abuse."`
}

const ChatGPTAPIURL = "https://api.openai.com/v1/chat/completions"

type ChatGPTClient struct {
	opts *ChatGPTOptions
	cli  *chat.Client[*Question, *Answer, *AnswerChunk]
}

func NewChatGPTClient(cli *http.Client, apikey string, opts *ChatGPTOptions) *ChatGPTClient {
	chatCli := chat.New[*Question, *Answer, *AnswerChunk](cli, ChatGPTAPIURL, apikey)
	return &ChatGPTClient{opts: opts, cli: chatCli}
}

func (c *ChatGPTClient) Ask(ctx context.Context, q *Question) (*Answer, error) {
	return c.cli.Ask(ctx, q)
}

func (c *ChatGPTClient) Stream(ctx context.Context, q *Question) (chan *AnswerChunk, error) {
	return c.cli.Stream(ctx, q)
}
