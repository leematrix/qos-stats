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

type BweStats struct {
	ThroughputEstimator float64 `json:"throughput"`
	ProbeEstimator      float64 `json:"probe"`
	DelayBasedBwe       float64 `json:"delay"`
	LossBasedBwe        float64 `json:"loss"`
	FinalBasedBwe       float64 `json:"final"`
	Rtt                 uint64  `json:"rtt"`
	RandomDelay         uint64  `json:"randomDelay"`
	LossRate            float64 `json:"lossRate"`
	RandomLossRate      float64 `json:"randomLossRate"`
	CreateTime          int64
	RealBandwidth       int
	RecvQueueLen        int
}

var bweMutex sync.Mutex
var realBandwidthKBPS int = 0
var recvQueueLen = 0
var bweChan = make(chan []byte, 1024)

var BweStatsQueue = make([]BweStats, 0)
var BweStatsMutex sync.RWMutex

func bweStatsStart() {
	var startTime int64 = 0
	for {
		select {
		case msg := <-bweChan:
			if startTime == 0 {
				startTime = time.Now().UnixMilli()
			}
			stats := BweStats{
				CreateTime: time.Now().UnixMilli(), //- startTime,
			}

			index := 0
			stats.ThroughputEstimator = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.ProbeEstimator = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.DelayBasedBwe = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.LossBasedBwe = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.FinalBasedBwe = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.Rtt = binary.BigEndian.Uint64(msg[index:])
			index += 8

			stats.LossRate = float64(msg[index]) / 255
			index++

			stats.RealBandwidth = realBandwidthKBPS
			stats.RecvQueueLen = recvQueueLen

			BweStatsMutex.Lock()
			BweStatsQueue = append(BweStatsQueue, stats)
			BweStatsMutex.Unlock()
		}
	}
}

func BweStatsIncoming(msg []byte) {
	bweChan <- msg
}

func BweStatsDraw() ([]byte, error) {
	now := time.Now().UnixMilli()
	xAxis := make([]string, 0)
	YAxis := make([][]opts.LineData, 5)

	BweStatsMutex.Lock()
	start := 0
	for i := 0; i < len(BweStatsQueue); i++ {
		if now-BweStatsQueue[i].CreateTime > conf.StatsWindowsTimeMs &&
			len(BweStatsQueue) > conf.StatsWindowsCount {
			start = i
		} else {
			break
		}
	}
	BweStatsQueue = BweStatsQueue[start:]
	BweStatsMutex.Unlock()

	BweStatsMutex.RLock()
	for _, stats := range BweStatsQueue {
		xAxis = append(xAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		YAxis[0] = append(YAxis[0], opts.LineData{Value: stats.RealBandwidth})
		YAxis[1] = append(YAxis[1], opts.LineData{Value: stats.ThroughputEstimator})
		YAxis[2] = append(YAxis[2], opts.LineData{Value: stats.ProbeEstimator})
		YAxis[3] = append(YAxis[3], opts.LineData{Value: stats.FinalBasedBwe})
		YAxis[4] = append(YAxis[4], opts.LineData{Value: stats.RecvQueueLen})
	}
	BweStatsMutex.RUnlock()

	var line = charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeShine,
			Width:  "1000px",
			Height: "500px"}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Qos Stats",
			Subtitle: "Bwe",
		}),
	)
	line.SetXAxis(xAxis).
		AddSeries("Real", YAxis[0]).
		AddSeries("Throughput", YAxis[1]).
		AddSeries("Probe", YAxis[2]).
		AddSeries("Target", YAxis[3]).
		AddSeries("Queue", YAxis[4]).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{Smooth: true, ShowSymbol: false}),
			charts.WithLabelOpts(opts.Label{Show: true}))
	line.Validate()
	return json.Marshal(line.JSON())
}

func BweStatsReset() {
	BweStatsMutex.Lock()
	defer BweStatsMutex.Unlock()
	BweStatsQueue = BweStatsQueue[:0]
	for len(bweChan) > 0 {
		select {
		case <-bweChan:
		default:
		}
	}
}
