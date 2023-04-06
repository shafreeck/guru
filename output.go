package main

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

type CommandOutput interface {
	io.Writer
	StylePrint(style lipgloss.Style, a ...any)
	StylePrintln(style lipgloss.Style, a ...any)
	StylePrintf(style lipgloss.Style, format string, a ...any)
	Print(a ...any)
	Println(a ...any)
	Printf(format string, a ...any)
	Error(a ...any)
	Errorln(a ...any)
	Errorf(format string, a ...any)
}

type commandStdout struct{}

func (out *commandStdout) Write(data []byte) (int, error) {
	return os.Stdout.Write(data)
}

func (out *commandStdout) StylePrint(style lipgloss.Style, a ...any) {
	var ss []any
	for _, s := range a {
		ss = append(ss, style.Render(fmt.Sprint(s)))
	}
	fmt.Fprint(os.Stdout, ss...)
}
func (out *commandStdout) StylePrintln(style lipgloss.Style, a ...any) {
	var ss []any
	for _, s := range a {
		ss = append(ss, style.Render(fmt.Sprint(s)))
	}
	fmt.Fprintln(os.Stdout, ss...)
}
func (out *commandStdout) StylePrintf(style lipgloss.Style, format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	rendered := style.Render(s)
	fmt.Fprint(os.Stdout, rendered)
}

func (out *commandStdout) Print(a ...any) {
	fmt.Print(a...)
}
func (out *commandStdout) Println(a ...any) {
	fmt.Println(a...)
}
func (out *commandStdout) Printf(format string, a ...any) {
	fmt.Printf(format, a...)
}

func (out *commandStdout) Error(a ...any) {
	fmt.Fprint(os.Stderr, a...)
}
func (out *commandStdout) Errorln(a ...any) {
	fmt.Fprintln(os.Stderr, a...)
}
func (out *commandStdout) Errorf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}
