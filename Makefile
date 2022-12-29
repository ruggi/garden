.PHONY: build
build:
	go run main.go -src ./Garden -dst ./out -tpl ./template.html
