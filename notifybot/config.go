package main

import (
	"os"
)

type Config struct {
	SlackToken, GithubToken string

	SlackChannel string

	// currently 1 supported, >5 not intended to be supoorted for now
	TriggerStrings []string

	CooldownDurationSeconds int64
}

func tmpConfig() Config {
	ghToken := os.Getenv("GITHUB_TOKEN")
	slackToken := os.Getenv("SLACK_TOKEN")

	if ghToken == "" {
		panic("missing GITHUB_TOKEN in env")
	}

	if slackToken == "" {
		panic("missing SLACK_TOKEN in env")
	}

	return Config{
		GithubToken: ghToken,
		SlackToken:  slackToken,

		SlackChannel:            "#bot-spam",
		TriggerStrings:          []string{"#snapmaintainers"},
		CooldownDurationSeconds: 5, // * 60,
	}
}
