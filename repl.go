package main

import (
	"github.com/chzyer/readline"
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

func repl(prompt string, do func(text string)) error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       prompt,
		AutoComplete: completer(complete),
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
		do(line)
	}
	return nil
}
