.PHONY: all clean

export GOPATH:=$(CURDIR)
export GOBIN:=$(CURDIR)/bin

all: recstation

recstation: cmd/recstation.go src/recstation/*.go src/mpeg/*.go
	go build $<

goget:
	go get recstation

test:
	go test mpeg

clean:
	rm -f recstation
