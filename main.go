package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
)

type set map[string]bool

var (
	src     string
	dst     string
	tplPath string
)

func init() {
	flag.StringVar(&src, "src", "", "where the notes are")
	flag.StringVar(&dst, "dst", "", "where the generated html is")
	flag.StringVar(&tplPath, "tpl", "", "path of the template")
}

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// parse the cmd flags
	flag.Parse()
	if src == "" {
		return fmt.Errorf("missing -src")
	}
	if dst == "" {
		return fmt.Errorf("missing -dst")
	}
	if tplPath == "" {
		return fmt.Errorf("missing -tpl")
	}

	// prep destination dir
	err := os.RemoveAll(dst)
	if err != nil {
		return fmt.Errorf("clean dst: %w", err)
	}
	err = os.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}

	// setup template
	tplData, err := os.ReadFile(tplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	tpl, err := template.New("template").Parse(string(tplData))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	// [[link]] regular expression
	reWikilinks, err := regexp.Compile(`\[{2}([a-zA-Z0-9- ]+)\]{2}`)
	if err != nil {
		return fmt.Errorf("compile regex: %w", err)
	}

	// make a map of all the valid filenames
	extensions := set{
		".md":  true,
		".png": true,
	}
	files := set{}
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk: %w", err)
		}
		if !extensions[filepath.Ext(path)] {
			return nil
		}
		relative := strings.TrimPrefix(
			filepath.Join(strings.TrimPrefix(filepath.Dir(path), filepath.Base(src)), filepath.Base(path)),
			"/",
		)
		files[relative] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk dir: %w", err)
	}

	data := map[string]string{}
	for f := range files {
		// read the file
		d, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			return fmt.Errorf("read path: %w", err)
		}
		data[f] = string(d)
	}

	// build incoming graph edges
	incoming := map[string]set{}
	for path, data := range data {
		// links := []string{}
		matches := reWikilinks.FindAllStringSubmatch(data, -1)
		for _, m := range matches {
			if incoming[m[1]] == nil {
				incoming[m[1]] = set{}
			}
			incoming[m[1]][path] = true
		}
	}

	// process the files
	for path, data := range data {
		// replace wikilinks
		for f := range files {
			f = strings.TrimSuffix(f, filepath.Ext(f))
			slug := strings.ReplaceAll(f, " ", "-")
			data = strings.ReplaceAll(data, "[["+f+"]]", `<a href="./`+slug+`.html">`+f+`</a>`)
		}
		data = reWikilinks.ReplaceAllString(data, `<a href="#">${1}</a>`)
		// process and output the file
		slug := strings.ReplaceAll(path, " ", "-")
		slugPath := filepath.Join(dst, slug)
		err = processFile(tpl, filepath.Base(path), slugPath, string(data), incoming)
		if err != nil {
			return fmt.Errorf("process %s: %s", path, err)
		}
	}

	return nil
}

type link struct {
	Href string
	Name string
}

type links []link

func (l links) Len() int           { return len(l) }
func (l links) Less(i, j int) bool { return l[i].Name < l[j].Name }
func (l links) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

type page struct {
	Title    string
	Body     string
	Incoming []link
}

func processFile(tpl *template.Template, name, path, data string, incoming map[string]set) error {
	base := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(name, filepath.Ext(name))
	baseWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	dstPath := filepath.Join(filepath.Dir(path), baseWithoutExt+".html")

	page := page{}
	page.Title = nameWithoutExt
	page.Body = string(markdown.ToHTML([]byte(data), nil, html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
	})))

	incomingLinks := links{}
	for f := range incoming[nameWithoutExt] {
		withoutExt := strings.TrimSuffix(f, filepath.Ext(f))
		incomingLinks = append(incomingLinks, link{
			Href: strings.ReplaceAll(withoutExt, " ", "-") + ".html",
			Name: withoutExt,
		})
	}
	sort.Sort(incomingLinks)
	page.Incoming = incomingLinks

	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		return err
	}

	f, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create %s: %s", dstPath, err)
	}
	defer f.Close()

	err = tpl.Execute(f, page)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
