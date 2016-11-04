package main

import (
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-github/github"
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

func main() {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: ""},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	_, xxx, _ := client.Zen()
	now, _ := time.Parse(time.RFC1123, xxx.Header.Get("Date"))
	fmt.Printf("Server time: %v\tLocal time: %v\n", now.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))

	timer := now
	seen := map[string]struct{}{}

	for {
		time.Sleep(2 * time.Second)
		sStr := "3hizc2QCWT in:title,body,comments" // updated:>" + timer.UTC().Format(time.RFC3339)
		res, _, _ := client.Search.Issues(sStr, &github.SearchOptions{
			TextMatch: true,
			Sort:      "updated",
			Order:     "desc",
		})
		fmt.Printf("%v results for:\t%v\n", len(res.Issues), sStr)
		for _, iss := range res.Issues {
			fmt.Println(*iss.HTMLURL, iss.UpdatedAt.Format(time.RFC3339))
			spew.Dump(iss.TextMatches)
			for _, frag := range iss.TextMatches {
				if _, ok := seen[*frag.ObjectURL]; ok {
					continue
				}
				seen[*frag.ObjectURL] = struct{}{}
				client.PullRequests.
					fmt.Printf("Message:\t%v\n", *frag.Fragment)
				fmt.Println()
			}
			if timer.Before(*iss.UpdatedAt) {
				timer = *iss.UpdatedAt
			}
		}
		fmt.Println("--------------------------------------------------------")
	}

}
