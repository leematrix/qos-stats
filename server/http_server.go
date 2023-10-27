package server

import (
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"qos-stats/biz"
	"strconv"
)

func bweIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("static/bwe.html")
	if err != nil {
		log.Fatal(err)
	}
	t.Execute(w, nil)
}

func costIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("static/cost.html")
	if err != nil {
		log.Fatal(err)
	}
	t.Execute(w, nil)
}

func trendIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("static/trend.html")
	if err != nil {
		log.Fatal(err)
	}
	t.Execute(w, nil)
}

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

func generateLineItems() []opts.LineData {
	items := make([]opts.LineData, 0)
	for i := 0; i < 7; i++ {
		items = append(items, opts.LineData{Value: rand.Intn(300)})
	}
	return items
}

func echartsTest(w http.ResponseWriter, _ *http.Request) {
	// create a new line instance
	line := charts.NewLine()

	// set some global options like Title/Legend/ToolTip or anything else
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWalden,
			Width:  "1800px",
			Height: "900px"}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Line example in Westeros theme",
			Subtitle: "Line chart rendered by the http server this time",
		}))

	// Put data into instance
	line.SetXAxis([]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}).
		AddSeries("Category A", generateLineItems()).
		AddSeries("Category B", generateLineItems()).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{Smooth: true, ShowSymbol: true}),
			charts.WithLabelOpts(opts.Label{Show: true}),
		)
	line.Render(w)
}

func HttpStart() {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", echartsTest)
	mux.HandleFunc("/bwe", bweIndex)
	mux.HandleFunc("/cost", costIndex)
	mux.HandleFunc("/trend", trendIndex)
	mux.HandleFunc("/stats", statsIndex)
	mux.HandleFunc("/stats/cost", statsCost)
	mux.HandleFunc("/stats/bwe", statsBwe)
	mux.HandleFunc("/stats/trend", statsTrend)
	mux.HandleFunc("/ws/stats", biz.WsStats)
	mux.HandleFunc("/ws/tc", biz.WsTc)

	s := &http.Server{
		Addr:    "0.0.0.0:8090",
		Handler: mux,
	}
	log.Println("Http Server Listen", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
