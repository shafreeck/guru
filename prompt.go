package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/shafreeck/guru/tui"
)

type AwesomePromptsRepo struct {
	Url    string
	Format string
	Saveas string
}

var AwesomePromptsRepos = []AwesomePromptsRepo{
	{Format: "csv", Saveas: "awesome-chatgpt-prompts.csv", Url: "https://raw.githubusercontent.com/f/awesome-chatgpt-prompts/main/prompts.csv"},
	{Format: "json", Saveas: "awesome-chatgpt-prompts-zh.json", Url: "https://raw.githubusercontent.com/PlexPt/awesome-chatgpt-prompts-zh/main/prompts-zh.json"},
}

type PromptEntry struct {
	Act    string
	Prompt string
}
type AwesomePrompts struct {
	dir string
	cli *http.Client
	out CommandOutput

	prompts []PromptEntry
}

func NewAwesomePrompts(dir string, cli *http.Client, out CommandOutput) *AwesomePrompts {
	if out == nil {
		out = &commandStdout{}
	}
	ap := &AwesomePrompts{dir: dir, cli: cli, out: out}
	ap.registerBuiltinCommands()
	return ap
}

func loadCSV(r io.Reader) []PromptEntry {
	var prompts []PromptEntry
	reader := csv.NewReader(r)
	for record, err := reader.Read(); err != io.EOF; record, err = reader.Read() {
		if err != nil {
			return nil
		}

		// ignore invalid lines
		if len(record) < 2 {
			continue
		}
		// ignore the file header
		if record[0] == "act" && record[1] == "prompt" {
			continue
		}

		prompts = append(prompts, PromptEntry{
			Act:    record[0],
			Prompt: record[1],
		})
	}
	return prompts
}

func loadJSON(r io.Reader) []PromptEntry {
	var prompts []PromptEntry

	decoder := json.NewDecoder(r)
	decoder.Decode(&prompts)

	return prompts
}

func (ap *AwesomePrompts) sync() error {
	for _, repo := range AwesomePromptsRepos {
		resp, err := ap.cli.Get(repo.Url)
		if err != nil {
			return err
		}
		r := resp.Body
		defer r.Close() // there would not have too much repos, so it's ok to use defer

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		saveas := path.Join(ap.dir, repo.Saveas)
		if err := os.WriteFile(saveas, data, 0644); err != nil {
			return err
		}
		ap.out.Println(repo.Url + " synced")
	}
	return nil
}

func (ap *AwesomePrompts) Load() error {
	prefix := "awesome-chatgpt-prompts"
	var prompts []PromptEntry

	entries, err := os.ReadDir(ap.dir)
	if err != nil {
		return err
	}

	var files []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			files = append(files, path.Join(ap.dir, entry.Name()))
		}
	}
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		r, err := os.Open(file)
		if err != nil {
			return err
		}
		defer r.Close()
		if path.Ext(file) == ".csv" {
			prompts = append(prompts, loadCSV(r)...)
		} else if path.Ext(file) == ".json" {
			prompts = append(prompts, loadJSON(r)...)
		}
	}
	ap.prompts = prompts
	return nil
}

func (ap *AwesomePrompts) actasCommand() string {
	opts := struct {
		Role []string `cortana:"role, -, -"`
	}{}

	if len(ap.prompts) == 0 {
		ap.out.Errorln("no prompts found, use ':prompt sync' to sync with the remote repo")
		return ""
	}

	if usage := builtins.Parse(&opts); usage {
		builtins.Usage()
		return ""
	}
	// reset the message list first
	builtins.Launch([]string{":reset"})

	var out, prompt string
	role := strings.Join(opts.Role, " ")
	for _, p := range ap.prompts {
		if p.Act != role {
			continue
		}
		prompt = p.Prompt
		out = fmt.Sprintf("***Role***: %s\n\n> %s\n\n", p.Act, tui.WrapWord([]byte(p.Prompt), 80))
		break // stop when matched
	}
	if out == "" {
		return ""
	}

	render := tui.MarkdownRender{}
	text, err := render.Render(out)
	if err != nil {
		ap.out.Errorln(err)
	}

	ap.out.Print(text)

	// return prompt to trigger a request
	return prompt
}
func (ap *AwesomePrompts) listCommand() (_ string) {
	render := tui.MarkdownRender{}

	var buf = bytes.NewBuffer(nil)
	for _, p := range ap.prompts {
		out := fmt.Sprintf("***Role***: %s\n\n> %s\n\n", p.Act, tui.WrapWord([]byte(p.Prompt), 80))
		text, err := render.Render(out)
		if err != nil {
			ap.out.Errorln(err)
			return
		}
		fmt.Fprint(buf, text)
	}
	tui.Display[tui.Model[string], string](context.Background(), tui.NewViewport("Awesome ChatGPT Prompts", buf.String()))
	return
}
func (ap *AwesomePrompts) syncCommand() (_ string) {
	if err := ap.sync(); err != nil {
		ap.out.Errorln(err)
		return
	}
	if err := ap.Load(); err != nil {
		ap.out.Errorln(err)
		return
	}
	return
}

func (ap *AwesomePrompts) registerBuiltinCommands() {
	builtins.AddCommand(":prompt act as", ap.actasCommand, "act as a role", ap.actasComplete)
	builtins.AddCommand(":prompt list", ap.listCommand, "list all prompts")
	builtins.AddCommand(":prompt sync", ap.syncCommand, "sync prompts with remote repos")
	builtins.Alias(":prompts", ":prompt list")
	builtins.Alias(":act as", ":prompt act as")
}

func (ap *AwesomePrompts) actasComplete(line []rune, pos int) ([][]rune, int) {
	n := 50 // return the first n prompts, TODO use a pager
	act := strings.TrimPrefix(string(line), ":act as ")
	var suggests [][]rune
	for i, p := range ap.prompts {
		if strings.HasPrefix(p.Act, act) {
			if i == n {
				break
			}
			suggests = append(suggests, []rune(strings.TrimPrefix(p.Act, act)))
		}
	}
	return suggests, len(act)
}
