package main

import (
	_ "net/http/pprof"

	"recstation"
)

func main() {
	recstation.RunMain()
}
