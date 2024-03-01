package biz

import (
	"encoding/binary"
	"encoding/json"
	"qos-stats/conf"
	"sync"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

type NseStats struct {
	Rtt           uint32 `json:"rtt"`
	SRtt          uint32 `json:"sRtt"`
	MinRtt        uint32 `json:"minRTT"`
	UpDelay       uint32 `json:"upDelay"`
	UpDelayJitter uint32 `json:"upDelayJitter"`
	SUpDelay      uint32 `json:"sUpDelay"`
	LossRate      uint8  `json:"lossRate"`
	Ts            int64  `json:"ts"`
	CreateTime    int64
	RecvQueueLen  int
}

var nseStatsChan = make(chan []byte, 1024)

var NseStatsQueue = make([]NseStats, 0)
var NseStatsMutex sync.RWMutex

func nseStatsStart() {
	var startTime int64 = 0
	for {
		select {
		case msg := <-nseStatsChan:
			if startTime == 0 {
				startTime = time.Now().UnixMilli()
			}
			stats := NseStats{
				CreateTime: time.Now().Unix(), //- startTime,
			}

			index := 0
			stats.Rtt = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.SRtt = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.MinRtt = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.UpDelay = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.SUpDelay = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.LossRate = msg[index]
			index += 1

			stats.Ts = int64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.RecvQueueLen = recvQueueLen

			NseStatsMutex.Lock()
			NseStatsQueue = append(NseStatsQueue, stats)
			NseStatsMutex.Unlock()
		}
	}
}

func NSEIncoming(msg []byte) {
	nseStatsChan <- msg
}

func NseStatsDraw() ([]byte, error) {
	xAxis := make([]string, 0)
	YAxis := make([][]opts.LineData, 5)

	NseStatsMutex.Lock()
	start := 0
	if len(NseStatsQueue) > conf.StatsWindowsCount {
		start = len(NseStatsQueue) - conf.StatsWindowsCount
	}

	NseStatsQueue = NseStatsQueue[start:]
	NseStatsMutex.Unlock()

	NseStatsMutex.RLock()
	for _, stats := range NseStatsQueue {
		xAxis = append(xAxis, time.Unix(stats.Ts, 0).Format("15:04:05"))
		YAxis[0] = append(YAxis[0], opts.LineData{Value: stats.Rtt})
		YAxis[1] = append(YAxis[1], opts.LineData{Value: stats.MinRtt})
		YAxis[2] = append(YAxis[2], opts.LineData{Value: stats.UpDelay})
		YAxis[3] = append(YAxis[3], opts.LineData{Value: stats.SUpDelay})
		YAxis[4] = append(YAxis[4], opts.LineData{Value: stats.LossRate})
	}
	NseStatsMutex.RUnlock()

	var line = charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeShine,
			Width:  "1000px",
			Height: "500px"}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Qos Stats",
			Subtitle: "NSE",
		}),
	)
	line.SetXAxis(xAxis).
		AddSeries("Rtt", YAxis[0]).
		AddSeries("MinRtt", YAxis[1]).
		AddSeries("UpDelay", YAxis[2]).
		AddSeries("SUpDelay", YAxis[3]).
		AddSeries("Loss", YAxis[4]).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{Smooth: true, ShowSymbol: false}),
			charts.WithLabelOpts(opts.Label{Show: true}))
	line.Validate()
	return json.Marshal(line.JSON())
}

func NseReset() {
	NseStatsMutex.Lock()
	defer NseStatsMutex.Unlock()
	NseStatsQueue = NseStatsQueue[:0]
	for len(nseStatsChan) > 0 {
		select {
		case <-nseStatsChan:
		default:
		}
	}
}
