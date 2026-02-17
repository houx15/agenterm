.PHONY: build run clean web-build frontend-build

build: frontend-build
	go build -o bin/agenterm ./cmd/agenterm

run:
	go run ./cmd/agenterm

frontend-build:
	npm --prefix frontend install
	npm --prefix frontend run build

web-build:
	npm --prefix web install
	npm --prefix web run build

clean:
	rm -rf bin/
