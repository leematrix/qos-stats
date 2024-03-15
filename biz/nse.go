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
	Rtt           uint32  `json:"rtt"`
	SRtt          uint32  `json:"sRtt"`
	MinRtt        uint32  `json:"minRTT"`
	UpDelay       int32   `json:"upDelay"`
	UpDelayJitter int32   `json:"upDelayJitter"`
	SUpDelay      int32   `json:"sUpDelay"`
	Slope         float64 `json:"slope"`
	Variance      float64 `json:"variance"`
	LossRate      uint8   `json:"lossRate"`
	Ts            int64   `json:"ts"`
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
				CreateTime: time.Now().UnixMilli(), //- startTime,
			}

			index := 0
			stats.Rtt = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.SRtt = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.MinRtt = binary.BigEndian.Uint32(msg[index:])
			index += 4

			stats.UpDelay = int32(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.SUpDelay = int32(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.Slope = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.Variance = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.LossRate = msg[index]
			index += 1

			stats.Ts = int64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.RecvQueueLen = recvQueueLen

			NseStatsMutex.Lock()
			NseStatsQueue = append(NseStatsQueue, stats)
			NseStatsMutex.Unlock()

			TraceIncoming(stats)
		}
	}
}

func NSEIncoming(msg []byte) {
	select {
	case nseStatsChan <- msg:
	default:
	}
}

func NseStatsDraw() ([]byte, error) {
	xAxis := make([]string, 0)
	YAxis := make([][]opts.LineData, 7)

	NseStatsMutex.Lock()
	start := 0
	if len(NseStatsQueue) > conf.StatsWindowsCount {
		start = len(NseStatsQueue) - conf.StatsWindowsCount
	}

	NseStatsQueue = NseStatsQueue[start:]
	NseStatsMutex.Unlock()

	NseStatsMutex.RLock()
	for _, stats := range NseStatsQueue {
		xAxis = append(xAxis, time.UnixMilli(stats.CreateTime).Format("15:04:05"))
		YAxis[0] = append(YAxis[0], opts.LineData{Value: stats.Rtt})
		YAxis[1] = append(YAxis[1], opts.LineData{Value: stats.MinRtt})
		YAxis[2] = append(YAxis[2], opts.LineData{Value: stats.UpDelay})
		YAxis[3] = append(YAxis[3], opts.LineData{Value: stats.SUpDelay})
		YAxis[4] = append(YAxis[4], opts.LineData{Value: stats.Slope})
		YAxis[5] = append(YAxis[5], opts.LineData{Value: stats.Variance})
		YAxis[6] = append(YAxis[6], opts.LineData{Value: stats.LossRate})
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
		AddSeries("Slope", YAxis[4]).
		AddSeries("Variance", YAxis[5]).
		AddSeries("Loss", YAxis[6]).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{Smooth: false, ShowSymbol: false}),
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
