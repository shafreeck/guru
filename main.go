package main

import (
	"encoding/json"
	"os"

	"github.com/chzyer/readline"
	"github.com/shafreeck/cortana"
	"gopkg.in/yaml.v3"
)

// guruCommand a shortcut for "guru chat", it checks the stdin,
// if it is a terminal, it shows the usage and exit.
// oterwise, it lanunches the "guru chat" command to handle the
// texts from stdin
func guruCommand() {
	if readline.IsTerminal(int(os.Stdin.Fd())) {
		cortana.Usage()
		return
	}
	New().ChatCommand()
}

func main() {
	g := New()
	unmarshaler := cortana.UnmarshalFunc(json.Unmarshal)
	cortana.AddConfig("guru.json", unmarshaler)                // deprecated
	cortana.AddConfig("~/.config/guru/guru.json", unmarshaler) // deprecated
	cortana.AddConfig("guru.yaml", cortana.UnmarshalFunc(yaml.Unmarshal))
	cortana.AddConfig("~/.guru/config", cortana.UnmarshalFunc(yaml.Unmarshal))
	cortana.Use(cortana.ConfFlag("--conf", "-c", unmarshaler))

	cortana.AddRootCommand(guruCommand)
	cortana.AddCommand("chat", g.ChatCommand, "chat with ChatGPT")
	cortana.AddCommand("config", g.ConfigCommand, "configure guru")
	cortana.AddCommand("serve ssh", g.ServeSSH, "serve as an ssh app")

	cortana.Alias("commit message", `chat --system "give me a one line commit message based on the diff with less than 15 words"`)
	cortana.Launch()
}
