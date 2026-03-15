.PHONY: test lint build cover clean

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	golangci-lint run

build:
	go build -o lazyspeed .

clean:
	rm -f lazyspeed coverage.out
