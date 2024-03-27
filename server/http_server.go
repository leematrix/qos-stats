package server

import (
	"html/template"
	"log"
	"net/http"
	"qos-stats/biz"
)

func statsIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("static/stats.html")
	if err != nil {
		log.Fatal(err)
	}
	t.Execute(w, nil)
}

func HttpStart() {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", statsIndex)
	mux.HandleFunc("/ws/stats", biz.WsStats)

	s := &http.Server{
		Addr:    "0.0.0.0:8090",
		Handler: mux,
	}
	log.Println("Http Server Listen", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
