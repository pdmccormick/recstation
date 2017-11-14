package recstation

import (
	"encoding/json"
	"log"
	"net/http"
)

func corsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func serveRoot(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)

	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	http.ServeFile(w, r, "index.html")
}

func serveStatus(state *State) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ch := make(chan *StatusMessage)

		state.StatusRequest <- ch
		status := <-ch

		corsHeaders(w)
		w.Header().Set("Content-Type", "application/json")

		enc := json.NewEncoder(w)
		if err := enc.Encode(status); err != nil {
			log.Print("Status:", err)
		}
	}
}

func serveRecord(state *State) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Bad method", http.StatusMethodNotAllowed)
			return
		}

		ch := make(chan bool)

		state.RecordRequest <- ch
		status := <-ch

		corsHeaders(w)
		w.Header().Set("Content-Type", "application/json")

		var resp struct {
			Success bool `json:"success"`
		}
		resp.Success = status

		enc := json.NewEncoder(w)
		if err := enc.Encode(resp); err != nil {
			log.Print("Record:", err)
		}
	}
}

func serveStop(state *State) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Bad method", http.StatusMethodNotAllowed)
			return
		}

		ch := make(chan bool)

		state.StopRequest <- ch
		status := <-ch

		corsHeaders(w)
		w.Header().Set("Content-Type", "application/json")

		var resp struct {
			Success bool `json:"success"`
		}
		resp.Success = status

		enc := json.NewEncoder(w)
		if err := enc.Encode(resp); err != nil {
			log.Print("Record:", err)
		}
	}
}

func StartWeb(state *State, addr string) error {
	http.HandleFunc("/", serveRoot)
	http.HandleFunc("/api/v1/status", serveStatus(state))
	http.HandleFunc("/api/v1/record", serveRecord(state))
	http.HandleFunc("/api/v1/stop", serveStop(state))

	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			panic(err)
		}
	}()

	return nil
}
