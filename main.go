package main

import (
	"encoding/json"

	"github.com/shafreeck/cortana"
	"gopkg.in/yaml.v3"
)

func main() {
	g := New()
	unmarshaler := cortana.UnmarshalFunc(json.Unmarshal)
	cortana.AddConfig("guru.json", unmarshaler)                // deprecated
	cortana.AddConfig("~/.config/guru/guru.json", unmarshaler) // deprecated
	cortana.AddConfig("guru.yaml", cortana.UnmarshalFunc(yaml.Unmarshal))
	cortana.AddConfig("~/.guru/config", cortana.UnmarshalFunc(yaml.Unmarshal))
	cortana.Use(cortana.ConfFlag("--conf", "-c", unmarshaler))

	cortana.AddRootCommand(g.ChatCommand)
	cortana.AddCommand("chat", g.ChatCommand, "chat with ChatGPT")
	cortana.AddCommand("config", g.ConfigCommand, "configure guru")
	cortana.AddCommand("serve ssh", g.ServeSSH, "serve as an ssh app")

	// Avoid using same word of command and prompt name, or it cause confused for cortana.
	// Ex. alias cheatsheet = "chat --prompt cheatsheet", when run with `chat --prompt cheatsheet`,
	// the part of `chat cheatsheet` will be recorgnized as `chat cheatsheet` alias, and `--prompt`
	// is the flag
	cortana.Alias("commit", `chat --prompt Committer`)
	cortana.Alias("cheat", `chat --prompt Cheatsheet`)
	cortana.Launch()
}
