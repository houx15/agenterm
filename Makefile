.PHONY: build run clean

build:
	go build -o bin/agenterm ./cmd/agenterm

run:
	go run ./cmd/agenterm

clean:
	rm -rf bin/
