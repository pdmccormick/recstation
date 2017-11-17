package recstation

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/elazarl/go-bindata-assetfs"
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

func servePreview(state *State) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		sink := q.Get("sink")
		nextVal := q.Get("next")

		ch := make(chan error)

		corsHeaders(w)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-control", "private, max-age=0, no-cache")

		req := PreviewMessage{
			Sink:   sink,
			Next:   (nextVal == "1"),
			Writer: w,
			Ready:  ch,
		}

		state.PreviewRequest <- req
		err := <-ch
		if err != nil {
			log.Printf("Preview error: %s", err)
		}
	}
}

func StartWeb(state *State, addr string) error {
	http.HandleFunc("/api/v1/status", serveStatus(state))
	http.HandleFunc("/api/v1/record", serveRecord(state))
	http.HandleFunc("/api/v1/stop", serveStop(state))
	http.HandleFunc("/api/v1/preview", servePreview(state))

	http.Handle("/", http.FileServer(
		&assetfs.AssetFS{
			Asset:     Asset,
			AssetDir:  AssetDir,
			AssetInfo: AssetInfo,
			Prefix:    "",
		}))

	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			panic(err)
		}
	}()

	return nil
}
