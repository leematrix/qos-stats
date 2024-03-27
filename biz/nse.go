package biz

import (
	"encoding/binary"
	"encoding/json"
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
	RecvQueueLen  int
}

var nseStatsChan = make(chan []byte, 1024)

var NseStatsQueue = make([]NseStats, 0)
var NseStatsMutex sync.RWMutex

func nseStatsStart() {
	for {
		select {
		case msg := <-nseStatsChan:
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

func RttStatsDraw() ([]byte, error) {
	NseStatsMutex.Lock()
	if len(NseStatsQueue) > conf.StatsWindowsCount {
		start := len(NseStatsQueue) - conf.StatsWindowsCount
		NseStatsQueue = NseStatsQueue[start:]
	}
	NseStatsMutex.Unlock()

	data := statsData{
		Series: [][]float64{{}, {}, {}, {}, {}, {}},
	}
	NseStatsMutex.RLock()
	for _, stats := range NseStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], float64(stats.Rtt))
		data.Series[1] = append(data.Series[1], float64(stats.MinRtt))
		data.Series[2] = append(data.Series[2], float64(stats.UpDelay))
		data.Series[3] = append(data.Series[3], float64(stats.SUpDelay))
		data.Series[4] = append(data.Series[4], stats.Slope)
		data.Series[5] = append(data.Series[5], stats.Variance)
	}
	NseStatsMutex.RUnlock()
	return json.Marshal(data)
}

func LossStatsDraw() ([]byte, error) {
	data := statsData{
		Series: [][]float64{{}},
	}
	NseStatsMutex.RLock()
	for _, stats := range NseStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], float64(stats.LossRate))
	}
	NseStatsMutex.RUnlock()
	return json.Marshal(data)
}

func RateStatsDraw() ([]byte, error) {
	data := statsData{
		Series: [][]float64{{}, {}},
	}
	NseStatsMutex.RLock()
	for _, stats := range NseStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], float64(stats.SendRate))
		data.Series[1] = append(data.Series[1], float64(stats.RecvRate))
	}
	NseStatsMutex.RUnlock()
	return json.Marshal(data)
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
