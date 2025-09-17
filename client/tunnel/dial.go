package tunnel

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/xmx/aegis-common/transport"
)

type Subscriber interface {
	OnDisconnected(err error, disconnAt time.Time)
	OnReconnected(mux transport.Muxer, disconnAt, connAt time.Time)
	OnExit(err error)
}

type DialConfig struct {
	Addresses  []string             // broker 地址
	DialConfig transport.DialConfig // Dial 配置
	Logger     *slog.Logger         // 日志
	Timeout    time.Duration        // 每次连接 broker 的超时时间。
	Handler    http.Handler
	Subscriber Subscriber
}

func (d DialConfig) Open() (Client, error) {
	if d.Timeout <= 0 {
		d.Timeout = 30 * time.Second
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.DialConfig.Parent == nil {
		d.DialConfig.Parent = context.Background()
	}
	if len(d.Addresses) == 0 {
		d.Addresses = []string{transport.BrokerHost}
	}

	size := len(d.Addresses)
	uniq := make(map[string]struct{}, size)
	addrs := make([]string, 0, size)
	for _, addr := range d.Addresses {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(addr, "443")
		}
		if _, exists := uniq[addr]; !exists {
			uniq[addr] = struct{}{}
			addrs = append(addrs, addr)
		}
	}
	d.Addresses = addrs

	ml := transport.NewMuxLoader(nil)
	ac := &agentClient{cfg: d, mux: ml}
	mux, err := ac.connect()
	if err != nil {
		return nil, err
	}
	go ac.serve(mux)

	return ac, nil
}
