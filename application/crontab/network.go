package crontab

import (
	"context"
	"net/http"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/library/httpkit"
	"github.com/xmx/aegis-common/system/network"
	"github.com/xmx/aegis-common/tunnel/tunconst"
)

func NewNetwork(cli httpkit.Client) cronv3.Tasker {
	return &networkCard{
		cli: cli,
	}
}

type networkCard struct {
	cli  httpkit.Client
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

	n.last = cards
	data := map[string]any{"data": cards}
	reqURL := tunconst.AgentToBroker("/api/system/network")
	strURL := reqURL.String()

	err := n.cli.SendJSON(ctx, http.MethodPost, strURL, nil, data, nil)
	if err != nil {
		return err
	}
	n.last = cards

	return nil
}
