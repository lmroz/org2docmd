package main

import (
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/google/go-querystring/query"
)

func getServerTime(client *github.Client) (time.Time, error) {
	_, resp, err := client.Zen()
	if err != nil {
		return time.Time{}, err
	}
	now, err := time.Parse(time.RFC1123, resp.Header.Get("Date"))
	if err != nil {
		return time.Time{}, err
	}
	return now.UTC(), nil

}

//slight reimplementation here to ease handling of comments

type comment struct {
	ID        *int       `json:"id,omitempty"`
	Body      *string    `json:"body,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	HTMLURL   *string    `json:"html_url,omitempty"`
}

// addOptions adds the parameters in opt as URL query parameters to s.  opt
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opt interface{}) (string, error) {
	v := reflect.ValueOf(opt)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opt)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

// https://developer.github.com/changes/2016-05-12-reactions-api-preview/
const mediaTypeReactionsPreview = "application/vnd.github.squirrel-girl-preview"

var hasCommentsRE = regexp.MustCompile(`^https:\/\/api\.github\.com\/repos\/[^\/]+\/[^\/]+\/(issues|pulls)\/[\d]+\/?$`)

func hasComments(url string) bool {
	return hasCommentsRE.MatchString(url)
}

func getComments(client *github.Client, url string, opt *github.IssueListCommentsOptions) ([]*comment, *github.Response, error) {
	url = strings.TrimSuffix(url, "/")
	url += "/comments"

	url, err := addOptions(url, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	comments := new([]*comment)
	resp, err := client.Do(req, comments)
	if err != nil {
		return nil, resp, err
	}

	return *comments, resp, err
}
