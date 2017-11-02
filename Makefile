.PHONY: all clean

export GOPATH:=$(CURDIR)
export GOBIN:=$(CURDIR)/bin

all: recstation

recstation: cmd/recstation.go src/recstation/*.go
	go build $<

goget:
	go get recstation

clean:
	rm -f recstation
