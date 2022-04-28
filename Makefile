VERSION=`git describe --tags --always`
BUILD=`date +%FT%T%z`
HASH=`git rev-parse --short HEAD`


LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD} -X main.gitCommit=${HASH}"

all: test build

build:
	go build -o wum ${LDFLAGS}

install:
	go install -o wum ${LDFLAGS}

clean:
	rm klogproc

test:
	go test ./...

.PHONY: clean install test
