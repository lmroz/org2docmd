package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	cfg := tmpConfig()

	if len(cfg.TriggerStrings) != 1 {
		panic("len(cfg.TriggerStrings) != 1 ")
	}

	debugIgnoreTime := false

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GithubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	lastTime, err := getServerTime(client)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Server time: %v\tLocal time: %v\n", lastTime.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))

	waitChan := time.After(time.Duration(cfg.CooldownDurationSeconds) * time.Second)

	mentions := NewMentions(cfg.SlackToken, cfg.SlackChannel)
MAIN_LOOP:
	for {
		<-waitChan
		waitChan = time.After(time.Duration(cfg.CooldownDurationSeconds) * time.Second)

		mentions.Clear()
		currentTime, err := getServerTime(client)

		inWindow := func(t time.Time) bool {
			return t.After(lastTime) && t.Before(currentTime)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "getting time: %v\n", err)

			// wait and retry without updating time
			time.Sleep(time.Minute)
			continue
		}

		sStr := cfg.TriggerStrings[0] + " in:title,body,comments"
		if !debugIgnoreTime {
			sStr += " updated:>" + lastTime.UTC().Format(time.RFC3339)
		}
		issues, _, err := client.Search.Issues(sStr, &github.SearchOptions{
			TextMatch: true,
			Sort:      "updated",
			Order:     "desc",
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "search issues: %v\n", err)

			// wait and retry without updating time
			time.Sleep(time.Minute)
			continue
		}

		for _, issue := range issues.Issues {
			if (strings.Contains(*issue.Body, cfg.TriggerStrings[0]) || strings.Contains(*issue.Title, cfg.TriggerStrings[0])) &&
				(inWindow(*issue.CreatedAt) || debugIgnoreTime) {
				mentions.Add(*issue.HTMLURL, *issue.Body)
			}
			if hasComments(*issue.URL) {
				optComments := &github.IssueListCommentsOptions{
					Sort:      "created",
					Direction: "asc",
				}
				comments, _, err := getComments(client, *issue.URL, optComments)

				if !debugIgnoreTime {
					optComments.Since = lastTime
				}

				if err != nil {
					fmt.Fprintf(os.Stderr, "retrieving %v: %v\n", *issue.URL, err)

					// wait and retry without updating time
					time.Sleep(time.Minute)
					continue MAIN_LOOP
				}

				for _, comment := range comments {
					if strings.Contains(*comment.Body, cfg.TriggerStrings[0]) &&
						(inWindow(*comment.CreatedAt) || debugIgnoreTime) {
						mentions.Add(*comment.HTMLURL, *comment.Body)
					}
				}
			}

			lastTime = currentTime

			mentions.Push()
		}
		fmt.Println("--------------------------------------------------------")
	}

}
