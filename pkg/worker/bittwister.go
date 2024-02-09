package worker

import (
	"net"
	"time"

	"github.com/celestiaorg/bittwister/xdp"
	"github.com/celestiaorg/bittwister/xdp/bandwidth"
	"github.com/celestiaorg/bittwister/xdp/latency"
	xdplatency "github.com/celestiaorg/bittwister/xdp/latency"
	"github.com/celestiaorg/bittwister/xdp/packetloss"
)

type BitTwister struct {
	networkInterface *net.Interface
	bw, pl, lt       xdp.XdpLoader
}

func NewBitTwister(networkInterface *net.Interface) *BitTwister {
	return &BitTwister{
		bw: &bandwidth.Bandwidth{
			NetworkInterface: networkInterface,
		},
		pl: &packetloss.PacketLoss{
			NetworkInterface: networkInterface,
		},
		lt: &latency.Latency{
			NetworkInterface: networkInterface,
		},
	}
}

// limit is in bytes per second
func (b *BitTwister) SetBandwidthLimit(limit int64) (xdp.CancelFunc, error) {
	if b.bw == nil {
		return nil, ErrBandwidthIsNil
	}

	bw := b.bw.(*bandwidth.Bandwidth)
	bw.Limit = limit
	return bw.Start()
}

// rate is in percentage (0-100)
func (b *BitTwister) SetPacketLossRate(rate int32) (xdp.CancelFunc, error) {
	if b.pl == nil {
		return nil, ErrPacketLossIsNil
	}

	pl := b.pl.(*packetloss.PacketLoss)
	pl.PacketLossRate = rate
	return pl.Start()
}

// if you wanna set only one of the latency or jitter, set the other to 0
func (b *BitTwister) SetLatencyAndJitter(latency, jitter time.Duration) (xdp.CancelFunc, error) {
	if b.lt == nil {
		return nil, ErrLatencyIsNil
	}
	lt := b.lt.(*xdplatency.Latency)

	lt.Latency = latency
	lt.Jitter = jitter
	return lt.Start()
}
