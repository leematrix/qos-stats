package biz

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"qos-stats/conf"
	"sync"
	"time"
)

type BweStats struct {
	AckedEstimator float64 `json:"acked"`
	ProbeEstimator float64 `json:"probe"`
	DelayBasedBwe  float64 `json:"delay"`
	LossBasedBwe   float64 `json:"loss"`
	FinalBasedBwe  float64 `json:"final"`
	Rtt            uint64  `json:"rtt"`
	RandomDelay    uint64  `json:"randomDelay"`
	LossRate       float64 `json:"lossRate"`
	RandomLossRate float64 `json:"randomLossRate"`
	CreateTime     int64
}

type BweStatsSession struct {
	BweStatsChan  chan []byte
	BweStatsQueue []BweStats
	BweStatsMutex sync.RWMutex
}

func BweStatsSessionCreate() *BweStatsSession {
	session := &BweStatsSession{
		BweStatsChan:  make(chan []byte, 1024),
		BweStatsQueue: make([]BweStats, 0),
	}
	return session
}

func (bwe *BweStatsSession) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("bwe exit.")
			return
		case msg := <-bwe.BweStatsChan:
			stats := BweStats{
				CreateTime: time.Now().Unix(),
			}

			index := 0
			stats.AckedEstimator = float64(binary.BigEndian.Uint64(msg[index:]))
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

			bwe.BweStatsMutex.Lock()
			bwe.BweStatsQueue = append(bwe.BweStatsQueue, stats)
			if len(bwe.BweStatsQueue) > conf.StatsWindowsCount {
				start := len(bwe.BweStatsQueue) - conf.StatsWindowsCount
				bwe.BweStatsQueue = bwe.BweStatsQueue[start:]
			}
			bwe.BweStatsMutex.Unlock()
		}
	}
}

func (bwe *BweStatsSession) Incoming(msg []byte) {
	select {
	case bwe.BweStatsChan <- msg:
	default:
	}
}

func (bwe *BweStatsSession) Draw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"Acked", "Probe", "Delay", "Loss", "Bandwidth"},
		Series:     [][]float64{{}, {}, {}, {}, {}},
		SeriesType: []string{"line", "line", "line", "line", "line"},
	}
	bwe.BweStatsMutex.RLock()
	for _, stats := range bwe.BweStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.AckedEstimator)
		data.Series[1] = append(data.Series[1], stats.ProbeEstimator)
		data.Series[2] = append(data.Series[2], stats.DelayBasedBwe)
		data.Series[3] = append(data.Series[3], stats.LossBasedBwe)
		data.Series[4] = append(data.Series[4], stats.FinalBasedBwe)
	}
	bwe.BweStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (bwe *BweStatsSession) Reset() {
	bwe.BweStatsMutex.Lock()
	defer bwe.BweStatsMutex.Unlock()
	bwe.BweStatsQueue = bwe.BweStatsQueue[:0]
	for len(bwe.BweStatsChan) > 0 {
		select {
		case <-bwe.BweStatsChan:
		default:
		}
	}
}
