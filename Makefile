.PHONY: run browser

run:
	go run *.go

ttm:
	go run *.go "ttm"

build-nocache:
	go build -a

runsm:
	./JohnnyCastaway2026 display 0:1728x1117

runbg:
	./JohnnyCastaway2026 display 1:1920x1080

runboth:
	./JohnnyCastaway2026 display 0:1728x1117 &
	./JohnnyCastaway2026 display 1:1920x1080

browser:
	go run main.go resource.go util.go uncompress.go browser.go browser