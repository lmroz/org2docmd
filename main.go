package main

import (
	"fmt"
	"html/template"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/google/go-querystring/query"
	"github.com/nlopes/slack"
	"golang.org/x/oauth2"
)

type Contrib struct {
	RepoName string
	RepoUrl  string
	Count    int
}

type Group struct {
	Slug string
	URL  string
}

type Contribs []Contrib

func (a Contribs) Len() int           { return len(a) }
func (a Contribs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Contribs) Less(i, j int) bool { return a[i].Count > a[j].Count }

type User struct {
	Name    string
	Avatar  string
	Login   string
	URL     string
	Mail    string
	Company string
	Groups  []Group
	Contr   Contribs
}

func eos(p *string) string {
	if p != nil {
		return *p
	} else {
		return ""
	}
}

type SortedUsers []User

func (a SortedUsers) Len() int      { return len(a) }
func (a SortedUsers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortedUsers) Less(i, j int) bool {
	if len(a[i].Groups) > len(a[j].Groups) {
		return true
	}
	if len(a[i].Groups) < len(a[j].Groups) {
		return false
	}

	if len(a[i].Contr) > 0 {
		if len(a[j].Contr) > 0 {
			return a[i].Contr[0].Count > a[j].Contr[0].Count
		} else {
			return true
		}
	} else {
		if len(a[j].Contr) > 0 {
			return false
		} else {
			return a[i].Name < a[j].Name
		}
	}
}

const entry = `###{{.Name}}
<img src="{{.Avatar}}" width="150px" />|[@{{.Login}}]({{.URL}})<br /><br />Mail: {{.Mail}} <br />Company: {{.Company}}|
---|:---|
- Groups:{{range $item := .Groups}}
    - [{{$item.Slug}}]({{$item.URL}})
{{end}}
{{if .Contr}}
- Contributions:{{range $item := .Contr}}
    - [{{$item.Count}}] [{{$item.RepoName}}]({{$item.RepoUrl}})
{{end}}{{end}}
----------
`

func main2() {
	db := map[string]*User{}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "CUT"},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	// xxx, resp, err := client.Repositories.ListCollaborators("intelsdi-x", "snap", nil)
	// fmt.Println(resp, err)
	// spew.Dump(xxx)
	// return

	teams, _, _ := client.Organizations.ListTeams("intelsdi-x", nil)

	///// populate users
	for _, team := range teams {
		tid := *team.ID
		tname := *team.Name
		if !strings.Contains(tname, "maint") {
			continue
		}
		users, _, _ := client.Organizations.ListTeamMembers(tid, nil)
		for _, user := range users {
			login := *user.Login
			u := db[login]
			if u == nil {
				u = &User{}
			}
			u.Login = login
			gr := Group{Slug: "@intelsdi-x/" + *team.Slug, URL: fmt.Sprintf("https://github.com/orgs/intelsdi-x/teams/%s", *team.Slug)}
			u.Groups = append(u.Groups, gr)
			db[login] = u
		}
	}

	for login := range db {
		usr, _, _ := client.Users.Get(login)
		db[login].Avatar = eos(usr.AvatarURL)
		db[login].Name = eos(usr.Name)
		if db[login].Name == "" {
			db[login].Name = login
		}
		db[login].Mail = eos(usr.Email)
		db[login].Company = eos(usr.Company)
		db[login].URL = eos(usr.HTMLURL)
	}

	//////////////// repo stats

	for _, repoType := range []string{"public", "private"} {

		repos, _, _ := client.Repositories.ListByOrg("intelsdi-x", &github.RepositoryListByOrgOptions{Type: repoType})
		for _, repo := range repos {
			if !strings.HasPrefix(*repo.Name, "snap") {
				continue
			}
			css, _, _ := client.Repositories.ListContributorsStats(*(*(*repo).Owner).Login, *repo.Name)
			for _, cs := range css {
				login := *cs.Author.Login
				_, ok := db[login]

				totalAdditions := 0
				for _, week := range cs.Weeks {
					totalAdditions += *week.Additions
				}

				if ok {
					contr := Contrib{Count: totalAdditions, RepoName: *repo.Name, RepoUrl: *repo.HTMLURL}
					if *repo.Private {
						contr.RepoName += " [private]"
					}
					db[login].Contr = append(db[login].Contr, contr)
				}
			}
		}
	}

	users := SortedUsers{}
	for _, usr := range db {
		sort.Sort(usr.Contr)
		users = append(users, *usr)
	}

	sort.Sort(users)

	tmpl := template.Must(template.New("Entry").Parse(entry))
	for _, user := range users {
		err := tmpl.Execute(os.Stdout, user)
		if err != nil {
			panic(err)
		}
	}
}

//slight reimplementation here

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

const TriggerString = "#snapmaintainers"
const CooldownDuration = time.Second * 3

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
	for _, mention := range self.transaction {
		fmt.Println(mention.Where, mention.Text)
		self.api.PostMessage(self.channel, mention.Where+"\n"+mention.Text, slack.NewPostMessageParameters())
	}
	self.Clear()
}

func NewMentions(slackAPIKey, channel string) *Mentions {
	res := new(Mentions)
	res.api = slack.New(slackAPIKey)
	res.channel = channel
	return res
}

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

func main() {
	debugIgnoreTime := false

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: },
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	lastTime, err := getServerTime(client)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Server time: %v\tLocal time: %v\n", lastTime.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))

	waitChan := time.After(CooldownDuration)

	mentions := NewMentions(, "#bot-spam")
MAIN_LOOP:
	for {
		<-waitChan
		waitChan = time.After(CooldownDuration)

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

		sStr := TriggerString + " in:title,body,comments"
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
			if (strings.Contains(*issue.Body, TriggerString) || strings.Contains(*issue.Title, TriggerString)) &&
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
					if strings.Contains(*comment.Body, TriggerString) &&
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
