package main

import (
	"strings"

	"github.com/shafreeck/cortana"
)

type Completion struct {
	*cortana.Cortana

	prefix   []rune
	suggests [][]rune
	pos      int
}

type CompleteCommand func(s []rune, pos int) ([][]rune, int)

var completes = &Completion{Cortana: cortana.New(cortana.ExitOnError(false))}

func (c *Completion) Launch(args []string) {
	cmd := c.SearchCommand(args)
	if cmd == nil {
		return
	}
	cmd.Proc()
}

func (c *Completion) Complete(line []rune, pos int) ([][]rune, int) {
	prefix := strings.TrimLeft(string(line), " ")
	trimed := strings.TrimSpace(string(line))
	completes := c.Cortana.Complete(trimed)
	switch len(completes) {
	case 0:
		fields := strings.Fields(trimed)
		cmd := c.Cortana.SearchCommand(fields)
		if cmd == nil {
			return nil, 0
		}
		// a command with args matched. Ex. compete hel[lo word]
		// if a "complete" command is registered, it should be call
		// to try to complete the args, witch returns "lo world", 3
		return c.RunComplete(cmd, line, pos)
	case 1:
		path := completes[0].Path
		// "com" matches "complete"
		if strings.HasPrefix(path, trimed) && path != trimed {
			return [][]rune{[]rune(strings.TrimPrefix(path, trimed))}, len(trimed)
		}
		// "complete" matches "complete"
		// return a space to at this "TAB" and
		// then execute the command at next "TAB"
		if !strings.HasSuffix(prefix, " ") && path == trimed {
			return [][]rune{[]rune(" ")}, 0
		}

		// a command matched
		return c.RunComplete(completes[0], line, pos)
	}

	var suggests [][]rune
	for _, c := range completes {
		path := c.Path
		if path == "" {
			continue
		}
		suggests = append(suggests, []rune(strings.TrimPrefix(path, prefix)))
	}
	return suggests, pos

}

func (c *Completion) RunComplete(cmd *cortana.Command, line []rune, pos int) ([][]rune, int) {
	c.prefix = line
	c.pos = pos

	// run the suggest command
	cmd.Proc()
	suggests := c.suggests
	pos = c.pos

	// reset the context
	c.suggests = nil
	c.pos = 0
	return suggests, pos
}

func (c *Completion) AddCommand(path string, cmd CompleteCommand) {
	c.Cortana.AddCommand(path, complete(cmd), "")
}

func complete(f func(s []rune, pos int) ([][]rune, int)) func() {
	return func() {
		suggests, pos := f(completes.prefix, completes.pos)
		completes.suggests = suggests
		completes.pos = pos
	}
}
