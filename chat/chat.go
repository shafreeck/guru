package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type Newer interface {
	New() any
}
type Question interface {
	Newer
	Marshal() ([]byte, error)
}
type Answer interface {
	Newer
	Unmarshal([]byte) error
}
type AnswerChunk interface {
	New() any
	Unmarshal([]byte) error
	SetError(err error)
}

func newObj[T Newer]() T {
	var t T
	return t.New().(T)
}

type AnswerChunks[A Answer] interface {
	Combine() A
}

type Error struct {
	Type    string
	Message string
}

type Chat[Q Question, A Answer, AC AnswerChunk] interface {
	Ask(ctx context.Context, q Q) (A, error)
	Stream(ctx context.Context, q Q) (chan AC, error)
}

type QuestionAnswer[Q Question, A Answer, AC AnswerChunk] struct {
	Question     Q
	Answer       A
	AnswerChunks []AC
}
type Client[Q Question, A Answer, AC AnswerChunk] struct {
	cli    *http.Client
	url    string
	apikey string
}

func New[Q Question, A Answer, AC AnswerChunk](cli *http.Client, url string, apikey string) *Client[Q, A, AC] {
	return &Client[Q, A, AC]{cli: cli, url: url, apikey: apikey}
}
func (c *Client[Q, A, _]) Ask(ctx context.Context, q Q) (A, error) {
	ans := newObj[A]()

	data, err := json.Marshal(q)
	if err != nil {
		return ans, err
	}

	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewBuffer(data))
	if err != nil {
		return ans, err
	}
	req.Header.Add("Authorization", "Bearer "+c.apikey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.cli.Do(req.WithContext(ctx))
	if err != nil {
		return ans, err
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return ans, err
	}
	if err := json.Unmarshal(data, &ans); err != nil {
		return ans, err
	}
	return ans, nil
}

func (c *Client[Q, _, AC]) Stream(ctx context.Context, q Q) (chan AC, error) {
	ch := make(chan AC)
	data, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.apikey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.cli.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		errbuf := bytes.NewBuffer(nil)
		for scanner.Scan() {
			ansc := newObj[AC]()
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			prefix := "data:"
			// it would be an error event if not data: prefixed
			if !strings.HasPrefix(line, prefix) {
				errbuf.WriteString(line)
				continue
			}
			if line == "data: [DONE]" {
				return
			}
			text := line[len(prefix):]

			if err := json.Unmarshal([]byte(text), ansc); err != nil {
				ansc.SetError(err)
			}

			select {
			case <-ctx.Done():
				return
			case ch <- ansc:
			}
		}

		if errbuf.Len() == 0 {
			return
		}
		// send the error message
		ansc := newObj[AC]()
		if err := json.Unmarshal(errbuf.Bytes(), ansc); err != nil {
			ansc.SetError(err)
		}
		select {
		case <-ctx.Done():
			return
		case ch <- ansc:
		}
	}()
	return ch, nil
}
