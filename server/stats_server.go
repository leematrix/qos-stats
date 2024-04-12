package server

import (
	"log"
	"net"
	"qos-stats/biz"
	"qos-stats/conf"
	"strconv"
)

var statsServerIP = "0.0.0.0"
var statsServerPort = 30000

func StatsServerStart() {
	sAddr, err := net.ResolveUDPAddr("udp", statsServerIP+":"+strconv.Itoa(statsServerPort))
	if err != nil {
		panic(err)
	}

	go biz.StatsSessMgr.CheckExpired()

	go func() {
		conn, listenErr := net.ListenUDP("udp", sAddr)
		if listenErr != nil {
			panic(listenErr)
		}
		defer conn.Close()
		log.Println("Stats server listen", sAddr)

		for {
			buf := make([]byte, 1024)
			_, _, err = conn.ReadFromUDP(buf)
			if err != nil {
				log.Println(err)
				continue
			}

			index := 0
			token := string(buf[:3])
			if token != conf.StatsToken {
				continue
			}
			index += 3

			sessionIdLen := buf[index]
			index++

			sessionID := string(buf[index : index+int(sessionIdLen)])
			index += int(sessionIdLen)

			sess := biz.StatsSessMgr.Get(sessionID)
			if sess == nil {
				sess = biz.StatsSessMgr.Add(sessionID)
			}

			switch buf[index] {
			case biz.TypeBweStats:
				sess.Bwe.Incoming(buf[index+1:])
			case biz.TypeTransportStats:
				//transportStatsIncoming(buf[1:])
			case biz.TypeFrameStats:
				//biz.FrameStatsIncoming(buf[index+1:])
			case biz.TypeDelayTrendStats:
				sess.Trend.Incoming(buf[index+1:])
			case biz.TypeNSEStats:
				sess.Nse.Incoming(buf[index+1:])
			case biz.TypeSenderTwoSecStats:
				sess.TwoSec.SenderIncoming(buf[index+1:])
			case biz.TypeReceiverTwoSecStats:
				sess.TwoSec.ReceiverIncoming(buf[index+1:])
			case biz.TypeReset:
				sess.Reset()
				log.Println("Reset stats.")
			}
		}
	}()
}
