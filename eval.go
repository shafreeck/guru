package main

import (
	"fmt"
	"strings"
	"unicode"
)

type EvalMode int

const (
	ChatEval EvalMode = iota
	SysCommandEval
	BuiltinCommandEval
)

type Evaluator struct {
	lp       *LivePrompt
	sess     *Session
	mode     EvalMode
	chatEval func(text string)
	suffixes []string // store the orignal LivePromot suffixes when switching mode
}

func NewEvaluator(sess *Session, lp *LivePrompt, chatEval func(text string)) *Evaluator {
	return &Evaluator{sess: sess, lp: lp, chatEval: chatEval}
}

func (e *Evaluator) eval(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// switch mode and return to another evaluation mode
	if e.switchMode(text) {
		return
	}

	switch e.mode {
	case SysCommandEval:
		sysCommandEval(e.sess, text)
	case BuiltinCommandEval:
		builtinCommandEval(e.sess, ":"+text)
	case ChatEval:
		e.chatEval(text)
	}
}

// switch mode according the value of text,
func (e *Evaluator) switchMode(text string) bool {
	old := e.mode
	defer func() {
		// restore the LivePromit suffixes
		if e.mode == ChatEval && old != ChatEval {
			e.lp.Suffixes = e.suffixes
		}
		// backup the suffixes
		if e.mode != ChatEval && old == ChatEval {
			e.suffixes = e.lp.Suffixes
		}
	}()

	// swith evaluation modes
	// $ to system command mode
	// : to builtin command mode
	// > to chat
	if text == "$" && old != SysCommandEval {
		e.lp.Delimiter = "$"
		e.mode = SysCommandEval
		return true
	}
	//if text == ":" && old != BuiltinCommandEval {
	//	e.lp.Delimiter = ":"
	//	e.mode = BuiltinCommandEval
	//	return true
	//}
	if text == ">" && old != ChatEval {
		e.lp.Delimiter = ">"
		e.mode = ChatEval
		return true
	}
	return false
}

func sysCommandEval(sess *Session, text string) (cont bool) {
	out, err := runCommand(text)
	if err != nil {
		sess.out.Error(err)
		return
	}
	fmt.Fprintln(sess.out, out)
	sess.Append(&Message{Role: User, Content: out})
	return
}

func builtinCommandEval(sess *Session, text string) (cont bool) {
	if len(text) == 0 {
		return false
	}
	args := strings.FieldsFunc(text, func() func(r rune) bool {
		arounded := false
		return func(r rune) bool {
			if r == '\'' || r == '"' {
				arounded = !arounded
				return true
			}
			if unicode.IsSpace(r) && !arounded {
				return true
			}
			return false
		}
	}())

	text = strings.TrimSpace(builtins.Launch(args))
	if text != "" {
		sess.Append(&Message{Role: User, Content: text})
		return true
	}
	return false
}
