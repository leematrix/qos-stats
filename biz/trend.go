package biz

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"qos-stats/conf"
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
	for {
		select {
		case msg := <-TrendStatsChan:
			stats := TrendStats{
				CreateTime: time.Now().Unix(),
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
	TrendStatsMutex.Lock()
	if len(TrendStatsQueue) > conf.StatsWindowsCount {
		start := len(TrendStatsQueue) - conf.StatsWindowsCount
		TrendStatsQueue = TrendStatsQueue[start:]
	}
	TrendStatsMutex.Unlock()

	data := statsData{
		Series: [][]float64{{}, {}, {}},
	}
	TrendStatsMutex.RLock()
	for _, stats := range TrendStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.Threshold)
		data.Series[1] = append(data.Series[1], stats.ModifiedTrend)
		data.Series[2] = append(data.Series[2], -stats.Threshold)
	}
	TrendStatsMutex.RUnlock()
	return json.Marshal(data)
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
