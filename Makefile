.PHONY: build
build:
	go build -o bin/release2chart main.go

test:
	go test -v ./...