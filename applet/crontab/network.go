package crontab

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/xmx/aegis-common/contract/message"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/library/httpx"
	"github.com/xmx/aegis-common/system/network"
	"github.com/xmx/aegis-common/transport"
)

func NewNetwork(parent context.Context, cli httpx.Client, log *slog.Logger) cronv3.Tasker {
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

func (n *networkCard) Info() cronv3.TaskInfo {
	return cronv3.TaskInfo{
		Name:      "上报网卡信息",
		Timeout:   10 * time.Second,
		Immediate: true,
		CronSched: cronv3.NewInterval(time.Minute),
	}
}

func (n *networkCard) Call(ctx context.Context) error {
	cards := network.Interfaces()
	if cards.Equal(n.last) {
		n.log.Debug("本机网卡未发生变化，无需上报")
		return nil
	}

	n.last = cards
	data := &message.Data[[]*network.Card]{Data: cards}
	reqURL := transport.NewBrokerURL("/api/system/network")
	strURL := reqURL.String()

	return n.cli.SendJSON(ctx, http.MethodPost, strURL, nil, data, nil)
}
