package main

import (
	"encoding/json"

	"github.com/shafreeck/cortana"
)

func main() {
	g := New()
	unmarshaler := cortana.UnmarshalFunc(json.Unmarshal)
	cortana.AddConfig("guru.json", unmarshaler)
	cortana.AddConfig("~/.config/guru/guru.json", unmarshaler)
	cortana.Use(cortana.ConfFlag("--conf", "-c", unmarshaler))

	cortana.AddCommand("chat", g.ChatCommand, "chat with ChatGPT")
	cortana.AddCommand("serve ssh", g.ServeSSH, "serve as an ssh app")

	cortana.Alias("commit message", `chat --system "give me a one line commit message based on the diff with less than 15 words"`)
	cortana.Launch()
}
