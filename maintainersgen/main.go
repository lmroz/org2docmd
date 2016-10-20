package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Contrib struct {
	RepoName string
	RepoURL  string
	Count    int
}
type Contribs []Contrib

func (a Contribs) Len() int           { return len(a) }
func (a Contribs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Contribs) Less(i, j int) bool { return a[i].Count > a[j].Count }

type Group struct {
	Slug    string
	URL     string
	Mention string
}

func (a Group) String() string {
	return a.Mention
}

type Groups []Group

func (a Groups) Len() int           { return len(a) }
func (a Groups) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Groups) Less(i, j int) bool { return a[i].Mention > a[j].Mention }
func (a Groups) LessAll(b Groups) bool {
	if len(a) != len(b) {
		return len(a) > len(b)
	}

	for iGr, gr := range a {
		if gr.Mention != b[iGr].Mention {
			return gr.Mention > b[iGr].Mention
		}
	}

	return false
}

type User struct {
	Name    string
	Avatar  string
	Login   string
	URL     string
	Mail    string
	Company string
	Groups  Groups
	Contr   Contribs
}

type Users []User

func (a Users) Len() int      { return len(a) }
func (a Users) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a Users) Less(i, j int) bool {

	if a[i].Groups.LessAll(a[j].Groups) {
		return true
	} else {
		if a[j].Groups.LessAll(a[i].Groups) {
			return false
		}
	}

	if len(a[i].Contr) > 0 {
		if len(a[j].Contr) > 0 {
			if a[i].Contr[0].Count != a[j].Contr[0].Count {
				return a[i].Contr[0].Count > a[j].Contr[0].Count
			}
		} else {
			return true
		}
	} else {
		if len(a[j].Contr) > 0 {
			return false
		}
	}
	return a[i].Name < a[j].Name
}

func StringOrDefault(p *string) string {
	if p != nil {
		return *p
	} else {
		return ""
	}
}

func main() {
	db := map[string]*User{}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	teams, _, err := client.Organizations.ListTeams(teamID, nil)
	if err != nil {
		panic(err)
	}
	allGroups := Groups{}
	///// populate users
	for _, team := range teams {
		tid := *team.ID
		tname := *team.Name
		if !strings.Contains(tname, "maintainers") {
			continue
		}
		gr := Group{Mention: "@" + teamID + "/" + *team.Slug,
			Slug: *team.Slug,
			URL:  fmt.Sprintf("https://github.com/orgs/"+teamID+"/teams/%s", *team.Slug),
		}

		allGroups = append(allGroups, gr)

		users, _, err := client.Organizations.ListTeamMembers(tid, nil)
		if err != nil {
			panic(err)
		}
		for _, user := range users {
			login := *user.Login
			u := db[login]
			if u == nil {
				u = &User{}
			}
			u.Login = login

			u.Groups = append(u.Groups, gr)
			allGroups = append(allGroups)
			db[login] = u
		}
	}
	sort.Sort(allGroups)

	for login := range db {
		usr, _, err := client.Users.Get(login)
		if err != nil {
			panic(err)
		}

		db[login].Avatar = StringOrDefault(usr.AvatarURL)
		db[login].Name = StringOrDefault(usr.Name)
		if db[login].Name == "" {
			db[login].Name = login
		}
		db[login].Mail = StringOrDefault(usr.Email)
		db[login].Company = StringOrDefault(usr.Company)
		db[login].URL = StringOrDefault(usr.HTMLURL)
	}

	//////////////// repo stats

	repoTypes := []string{"public", "private"}
	if !showPrivateRepos {
		repoTypes = []string{"public"}
	}

	for _, repoType := range repoTypes {

		repos, _, err := client.Repositories.ListByOrg(teamID, &github.RepositoryListByOrgOptions{Type: repoType})
		if err != nil {
			panic(err)
		}

		for _, repo := range repos {
			if !strings.HasPrefix(*repo.Name, "snap") {
				continue
			}

			var css []*github.ContributorStats
			var resp *github.Response
			// github may need some time to calculate stats
			for retries := 0; retries < 10; retries++ {
				css, resp, err = client.Repositories.ListContributorsStats(*(*(*repo).Owner).Login, *repo.Name)
				if resp != nil && resp.StatusCode == 202 {
					time.Sleep(2 * time.Second)
					continue
				}
				if err == nil {
					break
				}
			}
			if err != nil {
				panic(err)
			}

			for _, cs := range css {
				login := *cs.Author.Login
				_, ok := db[login]

				totalAdditions := 0
				for _, week := range cs.Weeks {
					totalAdditions += *week.Additions
				}

				if ok {
					contr := Contrib{Count: totalAdditions, RepoName: *repo.Name, RepoURL: *repo.HTMLURL}
					if *repo.Private {
						contr.RepoName += " [private]"
					}
					db[login].Contr = append(db[login].Contr, contr)
				}
			}
		}
	}

	users := Users{}
	for _, usr := range db {
		if _, ok := skipUsers[usr.Login]; ok {
			continue
		}
		sort.Sort(usr.Contr)
		sort.Sort(usr.Groups)
		users = append(users, *usr)
	}

	sort.Sort(users)

	// delimiters will be present at indices where user is member of different
	// groups than previous user
	delims := map[int]bool{}
	stars := map[int]bool{}
	for i, usr := range users {
		if i > 0 && (usr.Groups.LessAll(users[i-1].Groups) || users[i-1].Groups.LessAll(usr.Groups)) {
			delims[i] = true
		}
		if !usr.Groups.LessAll(allGroups) && !allGroups.LessAll(usr.Groups) {
			stars[i] = true
		}
	}

	// fmt.Print(entriesBegin)
	tmpl := template.Must(template.New("Maintainers").Parse(maintainersTemplate))
	err = tmpl.Execute(os.Stdout, struct {
		Users             []User
		Delims, AllGroups map[int]bool
	}{Users: users, Delims: delims, AllGroups: stars})
	if err != nil {
		panic(err)
	}
}
