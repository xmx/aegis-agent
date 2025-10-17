package clientd

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"time"

	"github.com/xmx/aegis-agent/machine"
	"github.com/xmx/aegis-common/contract/authmesg"
	"github.com/xmx/aegis-common/library/timex"
	"github.com/xmx/aegis-common/options"
	"github.com/xmx/aegis-common/tunnel/tundial"
	"github.com/xmx/aegis-common/tunnel/tunutil"
)

func Open(cfg tundial.Config, opts ...options.Lister[option]) (tundial.Muxer, error) {
	if cfg.Parent == nil {
		cfg.Parent = context.Background()
	}
	opt := options.Eval(opts...)

	ac := &agentClient{cfg: cfg, opt: opt}
	mux, err := ac.opens()
	if err != nil {
		return nil, err
	}

	amux := tundial.MakeAtomic(mux)
	ac.mux = amux
	go ac.serve(mux)

	return amux, nil
}

type agentClient struct {
	cfg tundial.Config
	opt option
	mux tundial.AtomicMuxer
}

func (ac *agentClient) Muxer() tundial.Muxer {
	return ac.mux
}

func (ac *agentClient) opens() (tundial.Muxer, error) {
	id, _ := machine.ID()
	req := &authmesg.AgentToBrokerRequest{
		MachineID: id,
		Goos:      runtime.GOOS,
		Goarch:    runtime.GOARCH,
		PID:       os.Getpid(),
		Args:      os.Args,
	}
	req.Workdir, _ = os.Getwd()
	req.Executable, _ = os.Executable()
	req.Hostname, _ = os.Hostname()
	if cu, _ := user.Current(); cu != nil {
		req.Username = cu.Username
	}

	var reties int
	startedAt := time.Now()
	for {
		reties++ // 连接次数累加

		mux, res, err := ac.open(req)
		if err == nil && res.Succeed() {
			return mux, nil
		}

		sleep := ac.backoff(time.Since(startedAt), reties)
		ac.log().Warn("等待重连", "reties", reties, "sleep", sleep, "error", err)
		if err = timex.Sleep(ac.cfg.Parent, sleep); err != nil {
			ac.log().Error("不满足继续重试连接的条件", "error", err)
			return nil, err
		}
	}
}

func (ac *agentClient) open(req *authmesg.AgentToBrokerRequest) (tundial.Muxer, *authmesg.BrokerToAgentResponse, error) {
	addresses := ac.cfg.Addresses
	mux, err := tundial.Open(ac.cfg)
	if err != nil {
		ac.log().Info("基础网络连接失败", "error", err, "addresses", addresses)
		return nil, nil, err
	}

	protocol, subprotocol := mux.Protocol()
	attrs := []any{
		slog.Any("local_addr", mux.Addr()),
		slog.Any("remote_addr", mux.RemoteAddr()),
		slog.Any("protocol", protocol),
		slog.Any("subprotocol", subprotocol),
		slog.Any("addresses", addresses),
	}
	ac.log().Info("基础网络连接成功", attrs...)

	res, err1 := ac.authentication(mux, req, time.Minute)
	if err1 != nil {
		attrs = append(attrs, slog.Any("error", err1))
	}
	if res != nil {
		attrs = append(attrs, slog.Any("auth_response", res))
	}
	if err1 == nil && res != nil && res.Succeed() {
		ac.log().Info("通道连接认证成功", attrs...)
		return mux, res, nil
	}

	if res != nil {
		attrs = append(attrs, slog.Any("agent_auth_response", res))
	}
	if err1 != nil {
		attrs = append(attrs, slog.Any("error", err1))
	}
	_ = mux.Close() // 关闭连接
	ac.log().Warn("基础网络连接成功但认证失败", attrs...)

	return nil, res, err1
}

func (ac *agentClient) authentication(mux tundial.Muxer, req *authmesg.AgentToBrokerRequest, timeout time.Duration) (*authmesg.BrokerToAgentResponse, error) {
	ctx, cancel := context.WithTimeout(ac.cfg.Parent, timeout)
	defer cancel()

	conn, err := mux.Open(ctx)
	if err != nil {
		return nil, err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer conn.Close()

	now := time.Now()
	_ = conn.SetDeadline(now.Add(timeout))
	if err = tunutil.WriteHead(conn, req); err != nil {
		return nil, err
	}
	resp := new(authmesg.BrokerToAgentResponse)
	err = tunutil.ReadHead(conn, resp)

	return resp, err
}

func (ac *agentClient) serve(mux tundial.Muxer) {
	srv := ac.opt.server
	if srv == nil {
		srv = &http.Server{Handler: http.NotFoundHandler()}
	}

	const sleep = 2 * time.Second
	for {
		err := srv.Serve(mux)
		_ = mux.Close()

		ac.log().Warn("通道掉线了", "error", err, "sleep", sleep)
		ctx := ac.cfg.Parent
		_ = timex.Sleep(ctx, sleep)
		if mux, err = ac.opens(); err != nil {
			break
		}

		ac.mux.Swap(mux)
	}
}

// backoff 通过持续连接耗费的时间和次数，计算出一个合理的重试时间。
func (ac *agentClient) backoff(elapsed time.Duration, reties int) time.Duration {
	if reties < 60 {
		return 3 * time.Second
	} else if reties < 200 {
		return 10 * time.Second
	} else if reties < 500 {
		return 20 * time.Second
	} else if reties < 1000 {
		return time.Minute
	}

	const mouth = 30 * 24 * time.Hour
	if elapsed < mouth {
		return time.Minute
	}

	return 10 * time.Minute
}

func (ac *agentClient) log() *slog.Logger {
	if l := ac.opt.logger; l != nil {
		return l
	}

	return slog.Default()
}
