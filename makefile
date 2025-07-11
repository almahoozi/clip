.PHONY: all build clean run test install

all: clean test

build:
	CGO_ENABLED=0 go build -ldflags "-X main.commit=`git rev-parse HEAD` -X main.ref=`git rev-parse --abbrev-ref HEAD` -X main.version=`git describe --tags --always`" -o ./bin/clip .

clean:
	rm -rf ./bin

run:
	go run -ldflags "-X main.commit=`git rev-parse HEAD` -X main.ref=`git rev-parse --abbrev-ref HEAD` -X main.version=`git describe --tags --always`" .

test:
	go test -v ./...

install:
	go install -ldflags "-X main.commit=`git rev-parse HEAD` -X main.ref=`git rev-parse --abbrev-ref HEAD` -X main.version=`git describe --tags --always`" .
	@echo "Installed clip to $$(go env GOPATH)/bin/clip"
