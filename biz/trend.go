package biz

import (
	"encoding/binary"
	"encoding/json"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"math"
	"qos-stats/conf"
	"strconv"
	"sync"
	"time"
)

type TrendStats struct {
	ModifiedTrend float64
	Threshold     float64
	CreateTime    int64
}

var TrendStatsChan = make(chan []byte, 1024)
var TrendStatsQueue = make([]TrendStats, 0)
var TrendStatsMutex sync.RWMutex

func trendStatsStart() {
	var startTime int64 = 0
	for {
		select {
		case msg := <-TrendStatsChan:
			if startTime == 0 {
				startTime = time.Now().UnixMilli()
			}
			stats := TrendStats{
				CreateTime: time.Now().Unix(), //- startTime,
			}

			index := 0
			stats.ModifiedTrend = math.Float64frombits(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.Threshold = math.Float64frombits(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			TrendStatsMutex.Lock()
			TrendStatsQueue = append(TrendStatsQueue, stats)
			TrendStatsMutex.Unlock()
		}
	}
}

func TrendStatsIncoming(msg []byte) {
	TrendStatsChan <- msg
}

func TrendStatsDraw() ([]byte, error) {
	//now := time.Now().UnixMilli()
	xAxis := make([]string, 0)
	YAxis := make([][]opts.LineData, 3)

	TrendStatsMutex.Lock()
	start := 0
	count := conf.StatsWindowsCount * 10000
	if len(BweStatsQueue) > count {
		start = len(BweStatsQueue) - count
	}
	/*
	   for i := 0; i < len(TrendStatsQueue); i++ {
	           if now-TrendStatsQueue[i].CreateTime > conf.StatsWindowsTimeMs &&
	                   len(TrendStatsQueue) > conf.StatsWindowsCount*1000 {
	                   start = i
	           } else {
	                   break
	           }
	   }
	*/
	TrendStatsQueue = TrendStatsQueue[start:]
	TrendStatsMutex.Unlock()

	TrendStatsMutex.RLock()
	for _, stats := range TrendStatsQueue {
		xAxis = append(xAxis, strconv.Itoa(int(stats.CreateTime)))
		YAxis[0] = append(YAxis[0], opts.LineData{Value: stats.Threshold})
		YAxis[1] = append(YAxis[1], opts.LineData{Value: stats.ModifiedTrend})
		YAxis[2] = append(YAxis[2], opts.LineData{Value: -stats.Threshold})
	}
	TrendStatsMutex.RUnlock()

	var line = charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeShine,
			Width:  "1000px",
			Height: "500px"}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Qos Stats",
			Subtitle: "Delay Trend",
		}),
	)
	line.SetXAxis(xAxis).
		AddSeries("ThresholdUpper", YAxis[0]).
		AddSeries("Trend", YAxis[1]).
		AddSeries("ThresholdLower", YAxis[2]).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{Smooth: true, ShowSymbol: false}),
			charts.WithLabelOpts(opts.Label{Show: true}))
	line.Validate()
	return json.Marshal(line.JSON())
}

func TrendStatsReset() {
	TrendStatsMutex.Lock()
	defer TrendStatsMutex.Unlock()
	TrendStatsQueue = TrendStatsQueue[:0]
	for len(TrendStatsChan) > 0 {
		select {
		case <-TrendStatsChan:
		default:
		}
	}
}
