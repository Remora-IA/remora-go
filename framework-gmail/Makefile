.PHONY: build clean test

build:
	go build -o gmail ./cmd/framework-gmail/

clean:
	rm -f gmail
	rm -rf temp/

test:
	go test ./...

dev:
	go run ./cmd/framework-gmail/
