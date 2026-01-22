package crontab

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xmx/aegis-agent/muxclient/rpclient"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/system/network"
)

func NewNetwork(cli rpclient.Client) cronv3.Tasker {
	return &networkCard{
		cli: cli,
	}
}

type networkCard struct {
	cli  rpclient.Client
	last network.Cards
}

func (n *networkCard) Info() cronv3.TaskInfo {
	return cronv3.TaskInfo{
		Name:      "上报网卡信息",
		Timeout:   10 * time.Second,
		Immediate: true,
		CronSched: cron.Every(time.Minute),
	}
}

func (n *networkCard) Call(ctx context.Context) error {
	cards := network.Interfaces()
	if cards.Equal(n.last) {
		return nil
	}

	data := n.convert(cards)
	if err := n.cli.PostNetworks(ctx, data); err != nil {
		return err
	}
	n.last = cards

	return nil
}

func (n *networkCard) convert(cards network.Cards) rpclient.NetworkCards {
	dats := make(rpclient.NetworkCards, 0, len(cards))
	for _, c := range cards {
		dat := &rpclient.NetworkCard{
			Name:  c.Name,
			Index: c.Index,
			MTU:   c.MTU,
			IPv4:  c.IPv4,
			IPv6:  c.IPv6,
			MAC:   c.MAC,
		}
		dats = append(dats, dat)
	}

	return dats
}
