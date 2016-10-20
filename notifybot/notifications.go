package main

import (
	"fmt"

	"github.com/nlopes/slack"
)

type Mention struct {
	Where, Text string
}

type Mentions struct {
	channel     string
	api         *slack.Client
	transaction []Mention
}

func (self *Mentions) Clear() {
	self.transaction = nil

}

func (self *Mentions) Add(where, text string) {
	self.transaction = append(self.transaction, Mention{Where: where, Text: text})
}

func (self *Mentions) Push() {
	params := slack.NewPostMessageParameters()
	params.AsUser = true
	params.UnfurlLinks = true
	params.UnfurlMedia = true
	for _, mention := range self.transaction {
		fmt.Println(mention.Where, mention.Text)
		self.api.PostMessage(self.channel, mention.Where, params)
	}
	self.Clear()
}

func NewMentions(slackAPIKey, channel string) *Mentions {
	res := new(Mentions)
	res.api = slack.New(slackAPIKey)
	res.channel = channel
	return res
}
