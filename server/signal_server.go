package server

import (
	"log"
	"net"
	"qos-stats/biz"
	"strconv"
)

var statsServerIP = "0.0.0.0"
var statsServerPort = 30000

func SignalServerStart() {
	sAddr, err := net.ResolveUDPAddr("udp", statsServerIP+":"+strconv.Itoa(statsServerPort))
	if err != nil {
		panic(err)
	}

	go func() {
		conn, err := net.ListenUDP("udp", sAddr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		log.Println("Stats server listen", sAddr)

		for {
			buf := make([]byte, 128)
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Println(err)
				continue
			}

			switch buf[0] {
			case biz.TypeBweStats:
				biz.BweStatsIncoming(buf[1:])
			case biz.TypeTransportStats:
				//transportStatsIncoming(buf[1:])
			case biz.TypeFrameStats:
				biz.FrameStatsIncoming(buf[1:])
			case biz.TypeDelayTrendStats:
				biz.TrendStatsIncoming(buf[1:])
			case biz.TypeNSEStats:
				biz.NSEIncoming(buf[1:])
			case biz.TypeReset:
				biz.NseReset()
				biz.BweStatsReset()
				biz.CostStatsReset()
				biz.TrendStatsReset()
				log.Println("Reset stats.")
			}
		}
	}()
}
