package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"recstation"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	recstation.RunMain()

}
