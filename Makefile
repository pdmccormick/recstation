.PHONY: all bin clean

export GOBIN:=$(shell go env GOPATH)/bin

all: bin bin/recstation

bin:
	mkdir -p bin

bin/recstation: cmd/recstation.go *.go mpeg/*.go
	go build -o $@ $<
	sudo setcap cap_net_raw+eip $@
	sudo setcap cap_net_admin+eip $@

bindata:
	$(GOBIN)/go-bindata -pkg recstation -o bindata.go -prefix html html/ html/css/ html/js

test:
	go test

clean:
	rm -rf ./bin/ ./bindata.go
