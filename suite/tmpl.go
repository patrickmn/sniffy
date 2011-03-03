package main

import (
	"fmt"
	"html/template"
	"path/filepath"
	"reflect"
)

const (
	templateDir = "tmpl"
)

var (
	templateLanguages = []string{ // with corresponding folders in templateDir
		"en",
	}
	templateFiles = []string{
		"header.html",
		"footer.html",
		"sidebar.html",
		"front.html",
		"auditor_dashboard.html",
		"auditor_interceptor.html",
		"proxy_dashboard.html",
		"proxy_settings.html",
		"proxyserver_selector.html",
	}
	templates     = map[string]*template.Template{}
	templateFuncs = template.FuncMap{
		"equal":     Equal,
		"summarize": Summarize,
	}
)

func loadTemplates() {
	for _, v := range templateLanguages {
		fnames := make([]string, len(templateFiles))
		for oi, ov := range templateFiles {
			fnames[oi] = filepath.Join(templateDir, v, ov)
		}
		debug.Println("Loading template set for language", v)
		t := template.New(v)
		t = template.Must(t.Funcs(templateFuncs).ParseFiles(fnames...))
		templates[v] = t
	}
}

func Summarize(x interface{}, l int) string {
	s := fmt.Sprintf("%s", x)
	if len(s) > l {
		return s[:l] + "..."
	}
	return s
}

func Equal(x interface{}, y interface{}) bool {
	return reflect.DeepEqual(x, y)
}
