.PHONY: build
build: recents
	go run main.go -src ./Garden -dst ./out -tpl ./template.html -r ./recents.txt

.PHONY: recents
recents:
	git log --name-only --pretty=format: -5 Garden | sort | uniq | grep '.md' | sort -u | sed 's/Garden\///' > recents.txt
