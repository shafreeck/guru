package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/shafreeck/guru/chat"
	"github.com/shafreeck/guru/tui"
)

type ChatOptions struct {
	ChatGPTOptions    `yaml:"chatgpt"`
	System            string `yaml:"system"`
	Oneshot           bool   `yaml:"oneshot"`
	OneshotPrompt     string `yaml:"-"` // the prompt for oneshot, submitted each time
	Verbose           bool   `yaml:"verbose"`
	NonInteractive    bool   `yaml:"non-interactive"`
	DisableAutoShrink bool   `yaml:"disable-auto-shrink"`
	Text              string `yaml:"-"`
}

type ChatCommand struct {
	c         chat.Chat[*Question, *Answer, *AnswerChunk]
	ap        *AwesomePrompts
	sess      *Session
	isVerbose bool
}

func NewChatCommand(sess *Session, ap *AwesomePrompts, httpCli *http.Client, opts *ChatCommandOptions) *ChatCommand {
	c := NewChatGPTClient(httpCli, opts.APIKey, &opts.ChatGPTOptions)
	return &ChatCommand{c: c, sess: sess, ap: ap, isVerbose: opts.Verbose}
}

func (c *ChatCommand) Talk(opts *ChatOptions) (string, error) {
	if opts.Oneshot {
		c.sess.ClearMessage()

		// submit prompt each time in oneshot mode
		if opts.OneshotPrompt != "" {
			p := c.ap.PromptText(opts.OneshotPrompt)
			if p == "" {
				return "", fmt.Errorf("prompt not found: %s", opts.OneshotPrompt)
			}
			c.sess.Append(&Message{Role: User, Content: p})
		}
	}

	if opts.Text != "" {
		c.sess.Append(&Message{Role: User, Content: opts.Text})
	}

	// return if there is nothing to ask
	if len(c.sess.Messages()) == 0 {
		return "", nil
	}

	if opts.Stream {
		return c.stream(context.Background(), opts)
	} else {
		return c.ask(context.Background(), opts)
	}
}
func (c *ChatCommand) verbose(text string) {
	if !c.isVerbose {
		return
	}
	c.sess.out.Println(text)
}
func (c *ChatCommand) ask(ctx context.Context, opts *ChatOptions) (string, error) {
	q := &Question{
		ChatGPTOptions: opts.ChatGPTOptions,
		Messages:       c.sess.Messages(),
	}
	ans, err := tui.Display[tui.Model[*Answer], *Answer](ctx,
		tui.NewSpinnerModel("thinking...", func() (*Answer, error) {
			return c.c.Ask(ctx, q)
		}))
	if err != nil {
		return "", err
	}

	// maybe ctrl+c interrupted
	if ans == nil {
		return "", nil
	}

	if ans.Error.Message != "" {
		return "", fmt.Errorf(ans.Error.Message)
	}

	out := bytes.NewBuffer(nil)
	for _, choice := range ans.Choices {
		content := strings.TrimSpace(choice.Message.Content)
		out.WriteByte('\n')
		out.WriteString(content)
		out.WriteByte('\n')

		c.sess.Append(choice.Message)
	}

	c.verbose("render with markdown")
	text, err := tui.Display[tui.Model[string], string](ctx, tui.NewMarkdownModel(out.String()))
	if err != nil {
		return "", err
	}

	// Print to output if the tui is not renderable
	// in case the the stdout is not terminal
	if !tui.IsRenderable() {
		c.sess.out.Print(text)
	}

	if !opts.NonInteractive {
		c.sess.out.Printf("Cost : prompt(%d) completion(%d) total(%d)",
			ans.Usage.PromptTokens, ans.Usage.CompletionTokens, ans.Usage.TotalTokens)
	}

	return text, nil
}

func (c *ChatCommand) stream(ctx context.Context, opts *ChatOptions) (string, error) {
retry:
	q := &Question{
		ChatGPTOptions: opts.ChatGPTOptions,
		Messages:       c.sess.Messages(),
	}
	// issue a request to the api
	s, err := tui.Display[tui.Model[chan *AnswerChunk], chan *AnswerChunk](ctx,
		tui.NewSpinnerModel("", func() (chan *AnswerChunk, error) {
			return c.c.Stream(ctx, q)
		}))
	if err != nil {
		return "", err
	}
	// ctrl+c interrupted
	if s == nil {
		return "", nil
	}

	// handle the stream and print the delta text, the whole
	// content is returned when finished
	content, err := tui.Display[tui.Model[string], string](ctx, tui.NewStreamModel(s, func(event *AnswerChunk) (string, error) {
		if event.Error.Message != "" {
			return "", fmt.Errorf("%s: %s", event.Error.Code, event.Error.Message)
		}
		if len(event.Choices) == 0 {
			return "", nil
		}
		return event.Choices[0].Delta.Content, nil
	}))

	// The token limit exceeded. auto shrink and retry if enabled
	if c.IsTokenExceeded(err) {
		if opts.DisableAutoShrink {
			return "", fmt.Errorf("%w\n\nUse `:messages shrink <expr>` to reduce the tokens", err)
		}

		n := c.sess.mm.autoShrink()

		// Nothing to shrink, return.
		// This is the case that the last message is large enough
		// to exceed the token limit.
		if n == 0 {
			return "", err
		}

		word := "message"
		if n > 1 {
			word = "messages"
		}
		c.sess.out.Printf("%d %s shrinked because of tokens limitation", n, word)
		goto retry
	}
	if err != nil {
		return "", err
	}

	// Print to output if the tui is not renderable
	// in case the the stdout is not terminal
	if !tui.IsRenderable() {
		c.sess.out.Print(content)
	}
	// append the response
	c.sess.Append(&Message{Role: Assistant, Content: content})

	return content, nil
}

func (c *ChatCommand) IsTokenExceeded(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "context_length_exceeded") {
		return true
	}
	return false
}
