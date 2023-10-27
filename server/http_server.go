package server

import (
	"encoding/json"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"qos-stats/biz"
	"strconv"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/gorilla/websocket"
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

type wsStatsReqMessage struct {
	StatsType string `json:"statsType"`
	StreamId  int    `json:"streamId"`
}

type wsStatsRespMessage struct {
	StatsType string `json:"statsType" `
	StreamId  int    `json:"streamId"`
	Payload   string `json:"payload"`

	Success bool `json:"success"` // 标志WebSocket请求是否成功，仅给Web客户端回复时有效
}

func wsStats(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		Subprotocols: []string{"json"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		}, // 关闭请求源地址检查
	}

	// 升级连接协议到WebSocket
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade to websocket fail: %s", err.Error())
		return
	}
	defer c.Close()

	respFun := func(statsType string, streamId int, payload string, success bool) {
		resp := wsStatsRespMessage{
			StatsType: statsType,
			StreamId:  streamId,
			Payload:   payload,
			Success:   success,
		}

		if result, resErr := json.Marshal(resp); resErr == nil {
			err = c.WriteMessage(1, result)
			if err != nil {
				log.Println("respErr", err)
			}
		} else {
			log.Println("resErr", resErr)
		}
	}

	log.Printf("New ws client, addr: %s", r.RemoteAddr)
	// 从WebSocket连接轮询消息
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("Ws read fail: %s", err.Error())
			break
		}

		log.Printf("Ws recv: %s", string(message))

		var statsMsg = wsStatsReqMessage{}
		err = json.Unmarshal(message, &statsMsg)
		if err != nil {
			log.Printf("Unmarshal fail: %s", err.Error())
			return
		}
		var respErr error = nil
		var respData []byte
		switch statsMsg.StatsType {
		case "stats":
			respData, respErr = biz.BweStatsDraw()
			respFun("bwe", -1, string(respData[:]), respErr == nil)
			respData, respErr = biz.TrendStatsDraw()
			respFun("trend", -1, string(respData[:]), respErr == nil)
			respData, respErr = biz.FrameCostStatsDraw(0)
			respFun("cost", 0, string(respData[:]), respErr == nil)
			respData, respErr = biz.FrameCostStatsDraw(1)
			respFun("cost", 1, string(respData[:]), respErr == nil)
		case "bwe":
			respData, respErr = biz.BweStatsDraw()
			respFun("bwe", -1, string(respData[:]), respErr == nil)
		case "trend":
			respData, respErr = biz.TrendStatsDraw()
			respFun("trend", -1, string(respData[:]), respErr == nil)
		case "cost":
			respData, respErr = biz.FrameCostStatsDraw(uint8(statsMsg.StreamId))
			respFun("cost", statsMsg.StreamId, string(respData[:]), respErr == nil)
		}

		if respErr != nil {
			log.Println("respErr", respErr)
		}
	}
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
	mux.HandleFunc("/ws/stats", wsStats)

	s := &http.Server{
		Addr:    "0.0.0.0:8090",
		Handler: mux,
	}
	log.Println("Http Server Listen", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
