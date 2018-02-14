.PHONY: all clean

export PATH:=$(CURDIR)/bin:$(PATH)
export GOPATH:=$(CURDIR)
export GOBIN:=$(CURDIR)/bin

all: recstation

recstation: cmd/recstation.go src/recstation/*.go src/mpeg/*.go bindata
	go build $<
	sudo setcap cap_net_raw+eip $@
	sudo setcap cap_net_admin+eip $@

goget:
	go get -u github.com/jteeuwen/go-bindata/...
	@make bindata
	go get recstation

bindata:
	go-bindata -pkg recstation -o src/recstation/bindata.go -prefix html html/ html/css/ html/js

test:
	go test mpeg

clean:
	rm -f ./recstation ./src/recstation/bindata.go
