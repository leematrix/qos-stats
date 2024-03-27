package biz

import (
	"encoding/binary"
	"encoding/json"
	"qos-stats/conf"
	"sync"
	"time"
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

var realBandwidthKBPS int = 0
var recvQueueLen = 0
var bweChan = make(chan []byte, 1024)

var BweStatsQueue = make([]BweStats, 0)
var BweStatsMutex sync.RWMutex

func bweStatsStart() {
	for {
		select {
		case msg := <-bweChan:
			stats := BweStats{
				CreateTime: time.Now().Unix(),
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
	BweStatsMutex.Lock()
	if len(BweStatsQueue) > conf.StatsWindowsCount {
		start := len(BweStatsQueue) - conf.StatsWindowsCount
		BweStatsQueue = BweStatsQueue[start:]
	}
	BweStatsMutex.Unlock()

	data := statsData{
		Series: [][]float64{{}, {}, {}},
	}
	BweStatsMutex.RLock()
	for _, stats := range BweStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.ThroughputEstimator)
		data.Series[1] = append(data.Series[1], stats.ProbeEstimator)
		data.Series[2] = append(data.Series[2], stats.FinalBasedBwe)
	}
	BweStatsMutex.RUnlock()
	return json.Marshal(data)
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
