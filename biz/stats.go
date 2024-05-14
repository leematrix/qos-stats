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
	TypeBweStats            = 0
	TypeTransportStats      = 1
	TypeFrameStats          = 2
	TypeDelayTrendStats     = 3
	TypeNSEStats            = 4
	TypeSenderTwoSecStats   = 5
	TypeReceiverTwoSecStats = 6
	TypeBwaStats            = 7
	TypeReset               = 255
)

type StatsSession struct {
	context   context.Context
	cancel    context.CancelFunc
	SessionID string
	Bwe       *BweStatsSession
	Trend     *TrendStatsSession
	Nse       *NseStatsSession
	TwoSec    *TwoSecStatsSession
	Bwa       *BwaStatsSession
}

func StatsSessionCreate(SessionID string) *StatsSession {
	session := &StatsSession{
		SessionID: SessionID,
		Bwe:       BweStatsSessionCreate(),
		Bwa:       BwaStatsSessionCreate(),
		Trend:     TrendStatsSessionCreate(),
		Nse:       NseStatsSessionCreate(),
		TwoSec:    TwoStatsSessionCreate(),
	}
	session.Start()
	return session
}

func (sess *StatsSession) Start() {
	sess.context, sess.cancel = context.WithCancel(context.Background())
	go sess.Bwe.Run(sess.context)
	go sess.Bwa.Run(sess.context)
	go sess.Trend.Run(sess.context)
	go sess.Nse.Run(sess.context)
	go sess.TwoSec.Run(sess.context)
}

func (sess *StatsSession) Stop() {
	sess.cancel()
}

func (sess *StatsSession) Reset() {
	sess.Bwe.Reset()
	sess.Trend.Reset()
	sess.Nse.Reset()
	sess.TwoSec.Reset()
}

func (sess *StatsSession) IsExpired() bool {
	return sess.Nse.IsExpired()
}

type StatsSessionManager struct {
	Sessions map[string]*StatsSession
	sync.RWMutex
}

var StatsSessMgr = StatsSessionManager{
	Sessions: make(map[string]*StatsSession),
}

func (mgr *StatsSessionManager) Add(SessionID string) *StatsSession {
	mgr.Lock()
	defer mgr.Unlock()
	sess := StatsSessionCreate(SessionID)
	StatsSessMgr.Sessions[SessionID] = sess
	log.Println("Add id:", SessionID)
	return sess
}

func (mgr *StatsSessionManager) Del(SessionID string) {
	mgr.Lock()
	defer mgr.Unlock()
	delete(StatsSessMgr.Sessions, SessionID)
}

func (mgr *StatsSessionManager) Get(SessionID string) *StatsSession {
	mgr.RLock()
	defer mgr.RUnlock()
	return StatsSessMgr.Sessions[SessionID]
}

func (mgr *StatsSessionManager) CheckExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				for id, sess := range mgr.Sessions {
					if sess.IsExpired() {
						sess.Stop()
						mgr.Del(id)
						log.Println("Del expired id:", id)
					}
				}
			}
		}
	}()
}

type wsStatsRespMessage struct {
	StatsType string `json:"statsType"`
	StreamId  int    `json:"streamId"`
	Payload   string `json:"payload"`

	Success bool `json:"success"` // 标志WebSocket请求是否成功，仅给Web客户端回复时有效
}

type statsData struct {
	Legend     []string    `json:"legend"`
	XAxis      []string    `json:"xAxis"`
	Series     [][]float64 `json:"series"`
	SeriesType []string    `json:"seriesType"`
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

	queryParams := r.URL.Query()
	// 获取id参数的值
	id := queryParams.Get("id")
	if id == "" {
		// 如果没有id参数，返回错误
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	// 打印id
	log.Printf("Received id: %s", id)

	sess := StatsSessMgr.Get(id)
	if sess == nil {
		log.Printf("Not exist id: %s", id)
		return
	}

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
				respData, respErr := sess.Bwe.Draw()
				err = respFun("Bwe", -1, string(respData[:]), respErr == nil)
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
				respData, respErr := sess.Trend.Draw()
				err = respFun("Trend", -1, string(respData[:]), respErr == nil)
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
				respData, respErr := sess.Nse.RttStatsDraw()
				err = respFun("Rtt", 255, string(respData[:]), respErr == nil)
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
				respData, respErr := sess.Nse.LossStatsDraw()
				err = respFun("Loss", -1, string(respData[:]), respErr == nil)
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
				respData, respErr := sess.Nse.RateStatsDraw()
				err = respFun("Rate", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// SendFrameRate
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.TwoSec.SenderFrameRateDraw()
				err = respFun("SendFrameRate", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// RecvFrameRate
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.TwoSec.ReceiverFrameRateDraw()
				err = respFun("RecvFrameRate", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// NackCount
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.TwoSec.NackCountDraw()
				err = respFun("NackCount", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// NackDuration
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.TwoSec.NackCostDraw()
				err = respFun("NackCost", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// SendSize
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.TwoSec.SendRateDraw()
				err = respFun("SendRate", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// SendCount
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.TwoSec.SendCountDraw()
				err = respFun("SendCount", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// RecvCount
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.TwoSec.RecvCountDraw()
				err = respFun("RecvCount", -1, string(respData[:]), respErr == nil)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	// Bwa
	eg.Go(func() error {
		for {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := sess.Bwa.Draw()
				err = respFun("Bwa", -1, string(respData[:]), respErr == nil)
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
