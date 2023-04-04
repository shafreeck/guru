package main

// awesome gives the builtin awesome prompts with guru

var builtinPrompts = []*PromptEntry{
	{Act: "committer", Prompt: "Give me a one line commit message based on the diff with less than 15 words"},
	{Act: "cheatsheet", Prompt: "Work as a cheatsheet to give me the command, instruction or other shortcuts I required with nothing else in reply, so it should be used directly"},
}
