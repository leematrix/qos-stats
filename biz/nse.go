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
	SendRate      int64   `json:"sendRate"`
	RecvRate      int64   `json:"recvRate"`
	Ts            int64   `json:"ts"`
	CreateTime    int64
}

type NseStatsSession struct {
	nseStatsChan  chan []byte
	NseStatsQueue []NseStats
	NseStatsMutex sync.RWMutex
	LastRecvMsg   int64
}

func NseStatsSessionCreate() *NseStatsSession {
	session := &NseStatsSession{
		nseStatsChan:  make(chan []byte, 1024),
		NseStatsQueue: make([]NseStats, 0),
		LastRecvMsg:   time.Now().Unix(),
	}
	return session
}

func (nse *NseStatsSession) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("nse exit.")
			return
		case msg := <-nse.nseStatsChan:
			stats := NseStats{
				CreateTime: time.Now().Unix(),
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

			stats.SendRate = int64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.RecvRate = int64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.Ts = int64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			nse.NseStatsMutex.Lock()
			nse.NseStatsQueue = append(nse.NseStatsQueue, stats)
			if len(nse.NseStatsQueue) > conf.StatsWindowsCount {
				start := len(nse.NseStatsQueue) - conf.StatsWindowsCount
				nse.NseStatsQueue = nse.NseStatsQueue[start:]
			}
			nse.NseStatsMutex.Unlock()

			TraceIncoming(stats)

			nse.LastRecvMsg = time.Now().Unix()
		}
	}
}

func (nse *NseStatsSession) Incoming(msg []byte) {
	select {
	case nse.nseStatsChan <- msg:
	default:
	}
}

func (nse *NseStatsSession) RttStatsDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"Rtt", "SRtt"},
		Series:     [][]float64{{}, {}},
		SeriesType: []string{"line", "line"},
	}
	nse.NseStatsMutex.RLock()
	for _, stats := range nse.NseStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], float64(stats.Rtt))
		data.Series[1] = append(data.Series[1], float64(stats.SRtt))
	}
	nse.NseStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (nse *NseStatsSession) LossStatsDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{""},
		Series:     [][]float64{{}},
		SeriesType: []string{"line"},
	}
	nse.NseStatsMutex.RLock()
	for _, stats := range nse.NseStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], float64(stats.LossRate))
	}
	nse.NseStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (nse *NseStatsSession) RateStatsDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"Send", "Recv"},
		Series:     [][]float64{{}, {}},
		SeriesType: []string{"line", "line"},
	}
	nse.NseStatsMutex.RLock()
	for _, stats := range nse.NseStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], float64(stats.SendRate))
		data.Series[1] = append(data.Series[1], float64(stats.RecvRate))
	}
	nse.NseStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (nse *NseStatsSession) Reset() {
	nse.NseStatsMutex.Lock()
	defer nse.NseStatsMutex.Unlock()
	nse.NseStatsQueue = nse.NseStatsQueue[:0]
	for len(nse.nseStatsChan) > 0 {
		select {
		case <-nse.nseStatsChan:
		default:
		}
	}
}

func (nse *NseStatsSession) IsExpired() bool {
	return time.Now().Unix()-nse.LastRecvMsg > 3600
}
