package crontab

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xmx/aegis-common/contract/message"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/library/httpx"
	"github.com/xmx/aegis-common/system/network"
	"github.com/xmx/aegis-common/transport"
)

func NewNetwork(parent context.Context, cli httpx.Client, log *slog.Logger) Tasker {
	return &networkCard{
		cli:    cli,
		log:    log,
		parent: parent,
	}
}

type networkCard struct {
	cli    httpx.Client
	log    *slog.Logger
	parent context.Context
	last   []*network.Card
}

func (n *networkCard) Spec() cron.Schedule {
	return cronv3.NewInterval(20 * time.Second)
}

func (n *networkCard) Call() {
	cards := network.Cards()
	if !n.isModified(cards) {
		n.log.Debug("网卡信息未发生变化")
		return
	}

	ctx, cancel := context.WithTimeout(n.parent, 10*time.Second)
	defer cancel()

	data := &message.Data[[]*network.Card]{Data: cards}
	reqURL := transport.NewBrokerURL("/api/system/network")
	strURL := reqURL.String()
	if err := n.cli.PostJSON(ctx, strURL, nil, data, struct{}{}); err != nil {
		n.log.Warn("上报网卡信息发生错误", slog.Any("error", err))
	}
}

func (n *networkCard) isModified(currents []*network.Card) bool {
	return true
}
