package biz

const (
	TypeBweStats        = 0
	TypeTransportStats  = 1
	TypeFrameStats      = 2
	TypeDelayTrendStats = 3
	TypeReset           = 255
)

func Start() {
	go bweStatsStart()
	go costStatsStart()
	go trendStatsStart()
}
