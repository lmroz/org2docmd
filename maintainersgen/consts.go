package main

import (
	"os"
)

const (
	entriesBegin = `#Snap Maintainers
	<table>
	    `
	entriesEnd = `</table>
	`

	// 	entryBasic = `###{{.Name}}
	// <img src="{{.Avatar}}" width="150px" />|[@{{.Login}}]({{.URL}})<br /><br />Mail: {{.Mail}} <br />Company: {{.Company}}|
	// ---|:---|
	// - Groups:{{range $item := .Groups}}
	//     - [{{$item.Mention}}]({{$item.URL}})
	// {{end}}
	// `

	entryBasic = `{{define "Mail"}}{{if .Mail}}<br /><b>Mail:</b> <a href="{{.Mail}}">{{.Mail}}</a>{{end}}{{end -}}
{{define "Company"}}{{if .Company}}<br /><b>Company:</b> {{.Company}}{{end}}{{end -}}
<tr><td colspan="3"><h4><a name="{{.Login}}" href="#{{.Login}}">#</a> {{.Name}}</h4></td></tr>
<td><img src="{{.Avatar}}" width="90px" /></td>
<td><b>Github:</b> <a href="{{.URL}}">@{{.Login}}</a><br />{{template "Mail" .}}{{template "Company" .}}</td>
<td><b>Groups:</b> <br /><br /><ul>{{range $item := .Groups}}
    <li><a href="{{$item.URL}}">{{$item.Mention}}</a></li>
{{end}}</ul></td></tr>
<tr><td colspan="3"></td></tr>
`

	entryContributionSection = `{{if .Contr}}
- Contributions:{{range $item := .Contr}}
    - [{{$item.Count}}] [{{$item.RepoName}}]({{$item.RepoURL}})
{{end}}{{end}}
`

	maintainersTemplate = `
{{- define "Groups" -}}
{{range $i, $item := .Groups}}{{if gt $i 0}}, {{end}}{{$item.Slug}}{{end}}
{{- end -}}
{{- define "EntryNG" -}}
{{.Name}} {{if .Mail}}<{{.Mail}}> {{end}}(@{{.Login}}) groups:
{{- end -}}

{{with $data := . -}}{{range $i, $item := $data.Users -}}
{{if index $data.Delims $i}}
{{end -}}
{{template "EntryNG" $item}} {{if index $data.AllGroups $i -}}
*{{else}}{{template "Groups" $item}}{{end}}
{{end}}{{end}}
`

	// 	entryTemplate = entryBasic +
	// 		entryContributionSection +
	// 		`----------
	// `

	entryTemplate    = entryBasic
	teamID           = "intelsdi-x"
	showPrivateRepos = false
)

var (
	githubToken = os.Getenv("GITHUB_TOKEN")

	skipUsers = map[string]struct{}{
		"marea-cobb": struct{}{},
		"snapbot":    struct{}{},
	}
)
