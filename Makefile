.PHONY: build run clean web-build

build:
	go build -o bin/agenterm ./cmd/agenterm

run:
	go run ./cmd/agenterm

web-build:
	npm --prefix web install
	npm --prefix web run build

clean:
	rm -rf bin/
