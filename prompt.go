package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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
	Sess    *session
	Dir     string
	Prompts []PromptEntry
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
	var prompts []PromptEntry
	for _, repo := range AwesomePromptsRepos {
		resp, err := httpClient.Get(repo.Url)
		if err != nil {
			return err
		}
		r := resp.Body
		defer r.Close() // there would not have too much repos, so it's ok to use defer

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		saveas := path.Join(ap.Dir, repo.Saveas)
		if err := os.WriteFile(saveas, data, 0644); err != nil {
			return err
		}
		fmt.Fprintln(tui.Stdout, blue.Render(repo.Url+" synced"))
	}
	ap.Prompts = prompts
	return nil
}

func (ap *AwesomePrompts) load() error {
	prefix := "awesome-chatgpt-prompts"
	var prompts []PromptEntry

	entries, err := os.ReadDir(ap.Dir)
	if err != nil {
		fmt.Println(err)
		return err
	}

	var files []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			files = append(files, path.Join(ap.Dir, entry.Name()))
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
	ap.Prompts = prompts
	return nil
}

func (ap *AwesomePrompts) actasCommand(ctx context.Context) string {
	opts := struct {
		Role []string `cortana:"role, -, -"`
	}{}

	if len(ap.Prompts) == 0 {
		fmt.Fprintln(tui.Stderr, red.Render("no prompts found, use ':prompt sync' to sync with the remote repo"))
		return ""
	}

	if usage := builtins.Parse(&opts); usage {
		builtins.Usage()
		return ""
	}
	// reset the message list first
	builtins.Launch(context.Background(), []string{":reset"})

	var out, prompt string

	role := strings.Join(opts.Role, " ")
	for _, p := range ap.Prompts {
		if p.Act != role {
			continue
		}
		ap.Sess.append(&Message{Role: User, Content: p.Prompt})
		prompt = p.Prompt
		out = fmt.Sprintf("***Role***: %s\n\n> %s\n\n", p.Act, tui.WrapWord([]byte(p.Prompt), 80))
		break // stop when matched
	}
	if out == "" {
		return ""
	}
	render := tui.MarkdownRender{}
	text, _ := render.Render(out)
	fmt.Fprint(tui.Stdout, string(text))
	return prompt
}
func (ap *AwesomePrompts) listCommand() {
	render := tui.MarkdownRender{}

	var buf = bytes.NewBuffer(nil)
	for _, p := range ap.Prompts {
		out := fmt.Sprintf("***Role***: %s\n\n> %s\n\n", p.Act, tui.WrapWord([]byte(p.Prompt), 80))
		text, err := render.Render(out)
		if err != nil {
			fmt.Fprint(tui.Stderr, red.Render(err.Error()))
			return
		}
		fmt.Fprint(buf, text)
	}
	tui.Display[tui.Model[string], string](context.Background(), tui.NewViewport("Awesome ChatGPT Prompts", buf.String()))
}
func (ap *AwesomePrompts) syncCommand() {
	if err := ap.sync(); err != nil {
		fmt.Fprintln(tui.Stdout, red.Render(err.Error()))
	}
	ap.load()
}

func (ap *AwesomePrompts) RegisterCommands() {
	builtins.AddCommand(":act as", builtin(ap.actasCommand), "act as a role")
	builtins.AddCommand(":prompt list", ap.listCommand, "list all prompts")
	builtins.AddCommand(":prompt sync", ap.syncCommand, "sync prompts with remote repos")
	builtins.Alias(":prompts", ":prompt list")
}

func ActAsComplete(line []rune, pos int) ([][]rune, int) {
	home, _ := os.UserHomeDir()
	ap := AwesomePrompts{Dir: path.Join(home, ".guru/prompt")}
	ap.load()

	prefix := string(line)
	if prefix == ":act as" {
		return [][]rune{[]rune(" ")}, 0
	}
	// the prefix should has prefix :act as
	if !strings.HasPrefix(prefix, ":act as ") {
		return nil, 0
	}

	n := 50 // return the first n prompts
	act := strings.TrimPrefix(prefix, ":act as ")
	var suggests [][]rune
	for i, p := range ap.Prompts {
		if strings.HasPrefix(p.Act, act) {
			if i == n {
				break
			}
			suggests = append(suggests, []rune(strings.TrimPrefix(p.Act, act)))
		}
	}
	return suggests, len(act)
}
