/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : dirlist.go

* Purpose :

* Creation Date : 08-23-2017

* Last Modified : Thu Aug 24 15:48:05 2017

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"github.com/dustin/go-humanize"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

const (
	staticTemplate = `
<form action="{{.Url}}" method="get">
  <input type="text" name="key" placeholder="Search..." autofocus><input type="submit" value="GO">
</form>
{{if .BackUrl}}<a href="{{.BackUrl}}"> &lt;- </a>{{end}}
<table>
  <tr>
    <th><a href="{{index .Urls "name"}}">Name</a></th>
    <th><a href="{{index .Urls "size"}}">Size</a></th> 
    <th><a href="{{index .Urls "lastMod"}}">LastMod</a></th>
  </tr>
  {{range .Files}}<tr>
    <td><a href="{{.Url}}">{{.Name}}</a></td>
    <td>{{.Size}}</td>
    <td>{{.LastMod}}</td>
  </tr>{{end}}
</table>`
)

type Page struct {
	Files   []*PageFile
	Url     string
	Urls    map[string]string
	BackUrl string
	Desc    string
}

type PageFile struct {
	Name    string
	Url     string
	Size    string
	LastMod string
}

func dirList1(w http.ResponseWriter, f http.File, u *url.URL) {
	dirs, err := f.Readdir(-1)
	if err != nil {
		// TODO: log err.Error() to the Server.ErrorLog, once it's possible
		// for a handler to get at its Server via the ResponseWriter. See
		// Issue 12438.
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}

	v := u.Query()
	orderBy := v.Get("by")
	desc := v.Get("desc")
	key := v.Get("key")
	var list []os.FileInfo
	if len(key) != 0 {
		for _, v := range dirs {
			if strings.Contains(v.Name(), key) {
				list = append(list, v)
			}
		}
		dirs = list
	}

	u.RawQuery = v.Encode()

	page := new(Page)
	page.Url = u.Path
	page.Urls = make(map[string]string)

	for _, t := range []string{"name", "size", "lastMod"} {
		v.Set("by", t)
		switch desc {
		case "1":
			v.Set("desc", "0")
		default:
			v.Set("desc", "1")
		}
		u.RawQuery = v.Encode()
		page.Urls[t] = u.String()
	}

	switch orderBy {
	case "name":
		sort.Slice(dirs, func(i, j int) bool {
			if desc == "0" {
				return dirs[i].Name() < dirs[j].Name()
			}
			return dirs[i].Name() > dirs[j].Name()
		})
	case "size":
		sort.Slice(dirs, func(i, j int) bool {
			if desc == "0" {
				return dirs[i].Size() < dirs[j].Size()
			}
			return dirs[i].Size() > dirs[j].Size()
		})
	default:
		sort.Slice(dirs, func(i, j int) bool {
			if desc == "0" {
				return dirs[i].ModTime().Unix() < dirs[j].ModTime().Unix()
			}
			return dirs[i].ModTime().Unix() > dirs[j].ModTime().Unix()
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	u := url.URL{Path: f.Name()}
	p := strings.Split(u.Path, "/")
	if len(p) > 2 {
		page.BackUrl = "/" + strings.Join(p[1:len(p)-2], "/")
	}
	for _, d := range dirs {
		var f PageFile
		f.Name = d.Name()
		if d.IsDir() {
			f.Name += "/"
		}
		f.Name = htmlReplacer.Replace(f.Name)
		u := url.URL{Path: f.Name}
		f.Url = u.String()
		f.Size = humanize.IBytes(uint64(d.Size()))
		f.LastMod = d.ModTime().Format("01-02-2006 15:04:05")
		// name may contain '?' or '#', which must be escaped to remain
		// part of the URL path, and not indicate the start of a query
		// string or fragment.
		// 		url := url.URL{Path: name}
		page.Files = append(page.Files, &f)
	}
	tmpl, err := template.New("page").Parse(staticTemplate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = tmpl.Execute(w, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
