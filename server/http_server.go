package server

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"qos-stats/biz"
	"strconv"
)

func statsIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("static/stats.html")
	if err != nil {
		log.Fatal(err)
	}
	t.Execute(w, nil)
}

func statsCost(w http.ResponseWriter, r *http.Request) {
	m, _ := url.ParseQuery(r.URL.RawQuery)
	streamId, _ := strconv.Atoi(m["stream_id"][0])
	data, err := biz.FrameCostStatsDraw(uint8(streamId))
	if err != nil {
		return
	}
	w.Write(data)
}

func statsBwe(w http.ResponseWriter, r *http.Request) {
	data, err := biz.BweStatsDraw()
	if err != nil {
		return
	}
	w.Write(data)
}

func statsTrend(w http.ResponseWriter, r *http.Request) {
	data, err := biz.TrendStatsDraw()
	if err != nil {
		return
	}
	w.Write(data)
}

func HttpStart() {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", statsIndex)
	mux.HandleFunc("/stats/cost", statsCost)
	mux.HandleFunc("/stats/bwe", statsBwe)
	mux.HandleFunc("/stats/trend", statsTrend)
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
