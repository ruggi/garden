.PHONY: build
build: recents
	go run main.go -src ./Garden -dst ./out -tpl ./template.html -r ./recents.txt

.PHONY: recents
recents:
	git diff --name-only -10 Garden | grep '.md' | sort -u | sed 's/Garden\///' > recents.txt
