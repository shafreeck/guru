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

type AwesomeRepo struct {
	Url    string
	Format string
	Saveas string
}

var defaultAwesomeRepos = []*AwesomeRepo{
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

	dict    map[string]*PromptEntry // dict for prompts
	repos   *AwesomeRepos
	prompts []*PromptEntry
}

func NewAwesomePrompts(dir string, cli *http.Client, out CommandOutput) *AwesomePrompts {
	if out == nil {
		out = &commandStdout{}
	}

	repos := &AwesomeRepos{filename: "awesome-repo.json"}
	ap := &AwesomePrompts{dir: dir, cli: cli, out: out,
		dict: make(map[string]*PromptEntry), repos: repos}
	repos.ap = ap

	ap.registerBuiltinCommands()
	return ap
}

func loadCSV(r io.Reader) []*PromptEntry {
	var prompts []*PromptEntry
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

		prompts = append(prompts, &PromptEntry{
			Act:    record[0],
			Prompt: record[1],
		})
	}
	return prompts
}

func loadJSON(r io.Reader) []*PromptEntry {
	var prompts []*PromptEntry

	decoder := json.NewDecoder(r)
	decoder.Decode(&prompts)

	return prompts
}

func (ap *AwesomePrompts) Load() error {
	prompts := builtinPrompts

	if err := ap.repos.load(); err != nil {
		return err
	}

	for _, repo := range ap.repos.repos {
		file := path.Join(ap.dir, repo.Saveas)
		r, err := os.Open(file)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		// ignore non-exist files, it requires the user to sync repos explicitely
		if os.IsNotExist(err) {
			continue
		}
		if path.Ext(file) == ".csv" {
			prompts = append(prompts, loadCSV(r)...)
		} else if path.Ext(file) == ".json" {
			prompts = append(prompts, loadJSON(r)...)
		}
		r.Close()
	}

	// Build the index
	for _, p := range prompts {
		ap.dict[p.Act] = p
	}

	ap.prompts = prompts
	return nil
}

func (ap *AwesomePrompts) PromptText(act string) string {
	p := ap.dict[act]
	if p != nil {
		return p.Prompt
	}
	return ""
}

func (ap *AwesomePrompts) actasCommand() string {
	opts := struct {
		Role []string `cortana:"role, -, -"`
	}{}

	if usage := builtins.Parse(&opts); usage {
		builtins.Usage()
		return ""
	}

	role := strings.Join(opts.Role, " ")

	p, ok := ap.dict[role]
	if !ok {
		ap.out.Errorln("prompt not found, use ':prompt repo sync' to sync with the remote repo")
		return ""
	}

	out := fmt.Sprintf("***Role***: %s\n\n> %s\n\n", p.Act, tui.WrapWord([]byte(p.Prompt), 80))
	text, err := tui.MarkdownRender{}.Render(out)
	if err != nil {
		ap.out.Errorln(err)
	}
	ap.out.Print(text)

	// reset the message list first
	builtins.Launch([]string{":reset"})

	// return prompt to trigger a request
	return p.Prompt
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
	if err := ap.repos.sync(); err != nil {
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
	builtins.AddCommand(":prompt repo sync", ap.syncCommand, "sync prompts with remote repos")
	builtins.AddCommand(":prompt repo add", ap.repos.addCommand, "add a remote repo")
	builtins.AddCommand(":prompt repo list", ap.repos.listCommand, "list remote repos")
	builtins.Alias(":repos", ":prompt repo list")
	builtins.Alias(":act as", ":prompt act as")
	builtins.Alias(":prompts", ":prompt list")
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

type AwesomeRepos struct {
	ap       *AwesomePrompts
	repos    []*AwesomeRepo
	filename string
}

func (ar *AwesomeRepos) save() error {
	file := path.Join(ar.ap.dir, ar.filename)
	data, err := json.Marshal(ar.repos)
	if err != nil {
		return err
	}
	if err := os.WriteFile(file, data, 0644); err != nil {
		return err
	}
	return nil
}
func (ar *AwesomeRepos) load() error {
	file := path.Join(ar.ap.dir, ar.filename)
	f, err := os.Open(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// initialize the repo file with builtin repos
	if os.IsNotExist(err) {
		ar.repos = defaultAwesomeRepos
		return ar.save()
	}
	defer f.Close()

	d := json.NewDecoder(f)
	if err := d.Decode(&ar.repos); err != nil {
		return err
	}

	return nil
}
func (ar *AwesomeRepos) sync() error {
	for _, repo := range ar.repos {
		resp, err := ar.ap.cli.Get(repo.Url)
		if err != nil {
			return err
		}
		r := resp.Body
		defer r.Close() // there would not have too much repos, so it's ok to use defer

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		saveas := path.Join(ar.ap.dir, repo.Saveas)
		if err := os.WriteFile(saveas, data, 0644); err != nil {
			return err
		}
		ar.ap.out.Println(repo.Url + " synced")
	}
	return nil
}

func (ar *AwesomeRepos) listCommand() (_ string) {
	for _, repo := range ar.repos {
		data, err := json.Marshal(repo)
		if err != nil {
			ar.ap.out.Errorln(err)
			return
		}

		text, err := (&tui.JSONRenderer{}).Render(string(data))
		if err != nil {
			ar.ap.out.Errorln(err)
			return
		}
		ar.ap.out.Println(text)
	}
	return
}

func (ar *AwesomeRepos) addCommand() (_ string) {
	opts := struct {
		Saveas string `cortana:"--saveas, -s,, filename to save"`
		Format string `cortana:"--format, -f,, format of file"`
		Url    string `cortana:"url, -,"`
	}{}

	if usage := builtins.Parse(&opts); usage {
		return
	}

	errln := ar.ap.out.Errorln
	switch {
	case opts.Url == "":
		errln("url is required")
		return
	case opts.Saveas == "":
		errln("--saveas is required")
		return
	case opts.Format == "":
		errln("--format is required")
		return
	}

	repo := AwesomeRepo{
		Saveas: opts.Saveas,
		Format: opts.Format,
		Url:    opts.Url,
	}
	ar.repos = append(ar.repos, &repo)

	if err := ar.save(); err != nil {
		ar.ap.out.Errorln(err)
		return
	}
	return
}
