package clientd

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/xmx/aegis-common/muxlink/muxconn"
	"github.com/xmx/aegis-common/muxlink/muxproto"
)

type Options struct {
	Semver  string
	Ident   Identifier
	Handler http.Handler
}

func (o Options) machineID(rebuild bool) string {
	if ident := o.Ident; ident != nil {
		return ident.MachineID(rebuild)
	}

	dir, _ := os.UserConfigDir()
	f := filepath.Join(dir, ".aegis-machine-id")
	ident := NewIdent(f)

	return ident.MachineID(rebuild)
}

func Open(cfg muxconn.DialConfig, opt Options) (muxconn.Muxer, error) {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	machineID := opt.machineID(false)

	req := &authRequest{
		MachineID: machineID,
		Semver:    opt.Semver,
		Goos:      runtime.GOOS,
		Goarch:    runtime.GOARCH,
		PID:       os.Getpid(),
		Args:      os.Args,
	}
	req.Workdir, _ = os.Getwd()
	req.Executable, _ = os.Executable()
	req.Hostname, _ = os.Hostname()

	mux := new(muxInstance)
	cli := &agentClient{
		cfg: cfg,
		opt: opt,
		mux: mux,
		req: req,
	}
	mc, err := cli.openLoop()
	if err != nil {
		return nil, err
	}
	mux.store(mc)

	go cli.serveHTTP()

	return mux, nil
}

type agentClient struct {
	cfg     muxconn.DialConfig
	opt     Options
	mux     *muxInstance
	req     *authRequest
	rebuild atomic.Bool
}

func (ac *agentClient) openLoop() (muxconn.Muxer, error) {
	var tires int
	for {
		tires++
		attrs := []any{"tires", tires}

		if mux, err := ac.open(); err != nil {
			attrs = append(attrs, "error", err)
		} else {
			ac.log().Info("节点上线成功")
			return mux, nil
		}

		du := ac.retryInterval(tires)
		attrs = append(attrs, "sleep", du)
		ac.log().Warn("通道连接失败，稍后重试", attrs...)

		if err := ac.sleep(du); err != nil {
			attrs = append(attrs, "final_error", err)
			ac.log().Error("通道连接遇到不可重试的错误", attrs...)
			return nil, err
		}
	}
}

//goland:noinspection GoUnhandledErrorResult
func (ac *agentClient) open() (muxconn.Muxer, error) {
	mux, err := muxconn.Open(ac.cfg)
	if err != nil {
		return nil, err
	}

	laddr, raddr := mux.Addr(), mux.RemoteAddr()
	outboundIP := muxproto.Outbound(laddr, raddr)
	ac.req.Inet = outboundIP.String()

	ctx, cancel := ac.perContext()
	defer cancel()

	conn, err := mux.Open(ctx)
	if err != nil {
		_ = mux.Close()
		return nil, err
	}
	defer conn.Close()

	timeout := ac.timeout()
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	if err = muxproto.WriteJSON(conn, ac.req); err != nil {
		_ = mux.Close()
		ac.log().Warn("写入认证消息错误", "error", err)
		return nil, err
	}

	resp := new(authResponse)
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	if err = muxproto.ReadJSON(conn, resp); err != nil {
		_ = mux.Close()
		ac.log().Warn("读取认证响应消息错误", "error", err)
		return nil, err
	}

	err = resp.checkError()
	if err == nil {
		return mux, nil
	}

	_ = mux.Close()
	if !resp.conflicted() {
		ac.log().Warn("节点认证失败", "error", err)
		return nil, err
	}
	if !ac.rebuild.CompareAndSwap(false, true) {
		ac.log().Warn("此机器码已经在线", "error", err)
		return nil, err
	}

	lastID := ac.req.MachineID
	machineID := ac.opt.machineID(true)
	if lastID != machineID {
		ac.req.MachineID = machineID
		ac.log().Warn("重新计算得到不同的机器码")
	} else {
		ac.log().Info("重新计算得到同样的机器码")
	}

	return nil, err
}

func (ac *agentClient) timeout() time.Duration {
	if d := ac.cfg.PerTimeout; d > 0 {
		return d
	}

	return time.Minute
}

func (ac *agentClient) parentContext() context.Context {
	if ctx := ac.cfg.Context; ctx != nil {
		return ctx
	}

	return context.Background()
}

func (ac *agentClient) perContext() (context.Context, context.CancelFunc) {
	d := ac.timeout()

	return context.WithTimeout(ac.parentContext(), d)
}

func (ac *agentClient) log() *slog.Logger {
	if l := ac.cfg.Logger; l != nil {
		return l
	}

	return slog.Default()
}

func (*agentClient) retryInterval(tires int) time.Duration {
	if tires <= 100 {
		return 3 * time.Second
	} else if tires <= 300 {
		return 10 * time.Second
	} else if tires <= 500 {
		return 30 * time.Second
	}

	return time.Minute
}

func (ac *agentClient) sleep(d time.Duration) error {
	ctx := ac.parentContext()
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (ac *agentClient) serveHTTP() {
	h := ac.opt.Handler
	if h == nil {
		h = http.NotFoundHandler()
	}

	for {
		srv := &http.Server{Handler: h}
		err := srv.Serve(ac.mux)
		ac.log().Warn("通道断开连接了", "error", err)

		mc, err1 := ac.openLoop()
		if err1 != nil {
			break
		}

		ac.mux.store(mc)
	}
}
