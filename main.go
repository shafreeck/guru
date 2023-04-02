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

	cortana.Alias("review", `chat --system 帮我Review以下代码,并给出优化意见,用Markdown渲染你的回应`)
	cortana.Alias("translate", `chat --system 帮我翻译以下文本到中文,用Markdown渲染你的回应`)
	cortana.Alias("unittest", `chat --system 为我指定的函数编写一个单元测试,用Markdown渲染你的回应`)
	cortana.Alias("commit message", `chat --system "give me a one line commit message based on the diff with less than 15 words"`)
	cortana.Launch()
}
