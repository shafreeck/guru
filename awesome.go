package main

// awesome gives the builtin awesome prompts with guru

var builtinPrompts = []*PromptEntry{
	{Act: "Committer", Prompt: "Give me a one line commit message using the imperative mood based on the diff with less than 10 words"},
	{Act: "Cheatsheet", Prompt: "Work as a cheatsheet to give me the command, instruction or other shortcuts that I required with nothing else in reply, so it could be used directly"},
}
