.PHONY: test lint build cover man clean

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	golangci-lint run

build:
	go build -o lazyspeed .

man:
	@go run . man --dir man/

clean:
	rm -f lazyspeed coverage.out
	rm -rf man/
