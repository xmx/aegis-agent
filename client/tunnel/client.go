package tunnel

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/xmx/aegis-common/contract/bamesg"
	"github.com/xmx/aegis-common/library/timex"
	"github.com/xmx/aegis-common/transport"
)

type Client interface {
	MuxLoader() transport.MuxLoader
	Close() error
}

type agentClient struct {
	cfg DialConfig
	mux transport.MuxLoader
}

func (ac *agentClient) MuxLoader() transport.MuxLoader {
	return ac.mux
}

func (ac *agentClient) Close() error {
	if mux, _ := ac.mux.LoadMux(); mux != nil {
		return mux.Close()
	}
	return nil
}

func (ac *agentClient) connect() (transport.Muxer, error) {
	req := bamesg.AgentRequest{
		MachineID: ac.cfg.MachineID,
		Goos:      runtime.GOOS,
		Goarch:    runtime.GOARCH,
	}

	var retry int
	timeout := ac.cfg.Timeout
	ctx := ac.context()
	startAt := time.Now()
	for {
		for _, addr := range ac.cfg.Addresses {
			mux, err := ac.open(addr, req, timeout)
			if err == nil {
				proto := mux.Protocol()
				ac.log().Info("连接中心端成功", "addr", addr, "protocol", proto)
				ac.mux.StoreMux(mux)
				return mux, nil
			}
			if exx := ctx.Err(); exx != nil {
				return nil, exx
			}

			retry++
			wait := ac.waitN(startAt, retry)

			attrs := []any{
				slog.Int("retry", retry),
				slog.Any("error", err),
				slog.Duration("wait", wait),
				slog.String("addr", addr),
			}
			ac.log().Warn("连接中心端失败", attrs...)

			if exx := timex.Sleep(ctx, wait); exx != nil {
				return nil, exx
			}
		}
	}
}

func (ac *agentClient) open(addr string, req bamesg.AgentRequest, timeout time.Duration) (transport.Muxer, error) {
	dc := ac.cfg.DialConfig
	mux, err := dc.DialTimeout(addr, timeout)
	if err != nil {
		return nil, err
	}

	if err = ac.handshake(mux, req, timeout); err != nil {
		_ = mux.Close()
		return nil, err
	}

	return mux, nil
}
func (ac *agentClient) handshake(mux transport.Muxer, req bamesg.AgentRequest, timeout time.Duration) error {
	parent := ac.context()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	sig, err := mux.Open(ctx)
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer sig.Close()

	deadline := time.Now().Add(timeout)
	_ = sig.SetDeadline(deadline)
	if err = json.MarshalEncode(jsontext.NewEncoder(sig), req); err != nil {
		return err
	}
	resp := new(bamesg.AgentResponse)
	if err = json.UnmarshalDecode(jsontext.NewDecoder(sig), resp); err != nil {
		return err
	}
	if resp.Succeed {
		return nil
	}

	return resp
}

func (ac *agentClient) context() context.Context {
	return ac.cfg.DialConfig.Parent
}

func (*agentClient) waitN(startAt time.Time, retry int) time.Duration {
	du := time.Since(startAt)
	if du < time.Minute {
		return time.Second
	} else if du < 5*time.Minute {
		return 3 * time.Second
	} else if du < 30*time.Minute {
		return 10 * time.Second
	} else {
		return time.Minute
	}
}

func (ac *agentClient) serve(mux transport.Muxer) {
	for {
		hand := ac.cfg.Handler
		srv := &http.Server{Handler: hand}
		err := srv.Serve(mux) // 连接正常会阻塞在这里
		_ = mux.Close()

		disconnAt := time.Now()
		sub := ac.cfg.Subscriber
		if sub != nil {
			sub.OnDisconnected(err, disconnAt)
		}

		ac.log().Warn("节点掉线了", slog.Any("error", err))
		if mux, err = ac.connect(); err != nil {
			ac.log().Error("连接失败，不再尝试重连", slog.Any("error", err))
			if sub != nil {
				sub.OnExit(err)
			}
			break
		} else {
			ac.log().Info("重连成功")
			connAt := time.Now()
			if sub != nil {
				sub.OnReconnected(mux, disconnAt, connAt)
			}
		}
	}
}

func (ac *agentClient) log() *slog.Logger {
	if l := ac.cfg.Logger; l != nil {
		return l
	}

	return slog.Default()
}
