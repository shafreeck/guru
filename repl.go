package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
	"github.com/shafreeck/guru/tui"
)

type completer func(line []rune, pos int) ([][]rune, int)

func (c completer) Do(line []rune, pos int) ([][]rune, int) {
	return c(line, pos)
}

func complete(line []rune, pos int) ([][]rune, int) {
	if len(line) == 0 {
		return nil, 0
	}
	switch line[0] {
	case '$':
		return cmdCompleter(line, pos)
	case ':':
		return builtinCompleter(line, pos)
	}
	return nil, 0
}

type livePrompt struct {
	append string // c is the string to append to the prompt
	count  int

	prompt string
	style  lipgloss.Style
}

func (live *livePrompt) push() {
	live.count++
}
func (live *livePrompt) pop() {
	live.count--
	if live.count < 0 {
		live.count = 0
	}
}
func (live *livePrompt) Render() string {
	s := strings.Repeat(live.append, live.count)
	return live.style.Render(live.prompt + s + " ")
}

func repl(prompt *livePrompt, do func(text string)) error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:         prompt.Render(),
		AutoComplete:   completer(complete),
		Stdin:          tui.Stdin,
		Stdout:         tui.Stdout,
		Stderr:         tui.Stderr,
		FuncIsTerminal: func() bool { return true },
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		// update the prompt for special command: < and >
		if len(line) > 0 || (len(line) > 1 && line[1] == ' ') {
			c := line[0]
			switch c {
			case '>':
				prompt.push()
				rl.SetPrompt(prompt.Render())
			case '<':
				prompt.pop()
				rl.SetPrompt(prompt.Render())
			}
		}
		do(line)
	}
	return nil
}
