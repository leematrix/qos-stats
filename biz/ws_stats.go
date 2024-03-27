package biz

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	TypeBweStats        = 0
	TypeTransportStats  = 1
	TypeFrameStats      = 2
	TypeDelayTrendStats = 3
	TypeNSEStats        = 4
	TypeReset           = 255
)

type wsStatsRespMessage struct {
	StatsType string `json:"statsType"`
	StreamId  int    `json:"streamId"`
	Payload   string `json:"payload"`

	Success bool `json:"success"` // 标志WebSocket请求是否成功，仅给Web客户端回复时有效
}

type statsData struct {
	XAxis  []string    `json:"xAxis"`
	Series [][]float64 `json:"series"`
}

func WsStats(w http.ResponseWriter, r *http.Request) {
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

	var mutex sync.Mutex
	respFun := func(statsType string, streamId int, payload string, success bool) error {
		resp := wsStatsRespMessage{
			StatsType: statsType,
			StreamId:  streamId,
			Payload:   payload,
			Success:   success,
		}

		if result, resErr := json.Marshal(resp); resErr == nil {
			mutex.Lock()
			err = c.WriteMessage(1, result)
			if err != nil {
				log.Println("respErr", err)
			}
			mutex.Unlock()
			return err
		} else {
			log.Println("resErr", resErr)
		}
		return nil
	}

	log.Printf("New ws client, addr: %s", r.RemoteAddr)

	eg, ctx := errgroup.WithContext(context.TODO())

	// Bwe
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := BweStatsDraw()
				err = respFun("bwe", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// Trend
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := TrendStatsDraw()
				err = respFun("trend", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// cost
	eg.Go(func() error {
		return nil
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := FrameCostStatsDraw(0)
				err = respFun("cost", 0, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
				respData, respErr = FrameCostStatsDraw(1)
				err = respFun("cost", 1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// Rtt
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := RttStatsDraw()
				err = respFun("rtt", 255, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// Loss
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := LossStatsDraw()
				err = respFun("loss", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// Rate
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := RateStatsDraw()
				err = respFun("rate", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	if err = eg.Wait(); err != nil {
		return
	}
}

func Start() {
	go TraceStart()
	go bweStatsStart()
	go costStatsStart()
	go trendStatsStart()
	go nseStatsStart()
}
