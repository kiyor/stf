/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : dirlist.go

* Purpose :

* Creation Date : 08-23-2017

* Last Modified : Wed 25 Oct 2017 10:32:57 PM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

var (
	extIcon = map[string]string{
		"mp4":  "file-video-o",
		"mov":  "file-video-o",
		"wmv":  "file-video-o",
		"avi":  "file-video-o",
		"flv":  "file-video-o",
		"go":   "file-code-o",
		"mp3":  "file-audio-o",
		"jpeg": "file-image-o",
		"jpg":  "file-image-o",
		"png":  "file-image-o",
		"gif":  "file-image-o",
	}
)

const (
	staticTemplate = `
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="referrer" content="none">
<meta name="google" content="notranslate">
<meta http-equiv="Content-Language" content="en">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.7.0/css/font-awesome.min.css">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-alpha.6/css/bootstrap.min.css" integrity="sha384-rwoIResjU2yc3z8GV/NPeZWAv56rSmLldC3R/AZzGRnGxQQKnKkoFVhFQhNUwEyJ" crossorigin="anonymous">
</head>
<style>
  body {
    font-family:"Microsoft Yahei","Helvetica Neue","Luxi Sans","DejaVu Sans",Tahoma,"Hiragino Sans GB",STHeiti;
  }
  table {
    font-size: 1.8em;
  }
  @media (max-width: 980px) {
    table {
      font-size: 1.8em;
    }
  }
</style>
<body>

<div class="container">
  <div class="row">
    <div class="col-1">
      {{if .BackUrl}}<a href="{{.BackUrl}}"><h1> &lt; </h1></a>{{end}}
    </div>
    <div class="col-9">
      <h1>{{.Title}}</h1>
    </div>
    <div class="col-2">
      <form action="{{.Url}}" method="get" class="bd-search hidden-sm-down">
        <input type="text" name="key" placeholder="Search..." value="{{.Key}}" autofocus>
      </form>
    </div>
  </div>
</div>

<div class="container">
  <div class="row">
    <div class="col-11">
      <table class="table table-hover">
        <tr>
          <th><a href="{{index .Urls "name"}}">Name</a></th>
          <th><a href="{{index .Urls "size"}}">Size</a></th> 
          <th><a href="{{index .Urls "lastMod"}}">LastMod</a></th>
        </tr>
        {{range .Files}}<tr>
          <td>{{.Icon}}  <a href="{{.Url}}">{{.Name}}</a></td>
          <td>{{.Size}}</td>
          <td>{{.LastMod}}</td>
        </tr>{{end}}
      </table>
    </div>
    <div class="col-1">
    </div>
  </div>
</div>

<div class="container">
  <div class="row">
    <div class="col-1">
      {{if .BackUrl}}<a href="{{.BackUrl}}"><h1> &lt; </h1></a>{{end}}
	</div>
    <div class="col-11">
	</div>
  </div>
</div>

</body>
</html>
`
)

type Page struct {
	Title   string
	Files   []*PageFile
	Url     string
	Urls    map[string]string
	Key     string
	BackUrl string
	Desc    string
}

type PageFile struct {
	Name    string
	Url     string
	Icon    template.HTML
	Size    string
	LastMod string
}

func dirList1(w http.ResponseWriter, f http.File, r *http.Request) {
	dirs, err := f.Readdir(-1)
	if err != nil {
		// TODO: log err.Error() to the Server.ErrorLog, once it's possible
		// for a handler to get at its Server via the ResponseWriter. See
		// Issue 12438.
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}

	v := r.URL.Query()
	orderBy := v.Get("by")
	desc := v.Get("desc")
	key := v.Get("key")
	var list []os.FileInfo
	if len(key) != 0 {
		for _, v := range dirs {
			if v.Name() == key {
				u := url.URL{Path: v.Name()}
				http.Redirect(w, r, u.String(), 302)
				return
			}
			if strings.Contains(v.Name(), key) {
				list = append(list, v)
			}
		}
		dirs = list
	}

	r.URL.RawQuery = v.Encode()

	page := new(Page)
	stat, _ := f.Stat()
	page.Title = stat.Name()
	if page.Title == "." {
		page.Title = "/"
	}
	page.Url = r.URL.String()
	page.Urls = make(map[string]string)
	page.Key = key

	for _, t := range []string{"name", "size", "lastMod"} {
		v.Set("by", t)
		switch desc {
		case "1":
			v.Set("desc", "0")
		default:
			v.Set("desc", "1")
		}
		// 		r.URL.RawQuery = v.Encode()
		// 		page.Urls[t] = r.url.string()
		page.Urls[t] = "?" + v.Encode()
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
	r.URL.RawQuery = ""
	p := strings.Split(r.URL.String(), "/")
	if len(p) > 2 {
		page.BackUrl = "/" + strings.Join(p[1:len(p)-2], "/") + "/"
		if strings.Contains(page.BackUrl, "//") {
			page.BackUrl = "/"
		}
	}
	for _, d := range dirs {
		var f PageFile
		f.Name = d.Name()
		if d.IsDir() {
			f.Name += "/"
			f.Icon = `<i class="fa fa-folder-open-o" aria-hidden="true"></i>`
		} else {
			f.Icon = getIcon(f.Name)
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

func getIcon(file string) template.HTML {
	p := strings.Split(file, ".")
	if len(p) > 1 {
		ext := p[len(p)-1:][0]
		if v, ok := extIcon[ext]; ok {
			return template.HTML(fmt.Sprintf(`<i class="fa fa-%s" aria-hidden="true"></i>`, v))
		}
	}
	return `<i class="fa fa-file-o" aria-hidden="true"></i>`
}
