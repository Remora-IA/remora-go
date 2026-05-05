.PHONY: build clean test

build:
	go build -o pingpong ./cmd/framework-pingpong/

clean:
	rm -f pingpong
	rm -rf temp/

test:
	go test ./...

dev:
	go run ./cmd/framework-pingpong/
