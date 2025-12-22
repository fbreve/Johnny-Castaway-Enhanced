.PHONY: run browser

run:
	go run *.go

ttm:
	go run *.go "ttm"

browser:
	go run main.go resource.go util.go uncompress.go browser.go browser