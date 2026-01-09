package clientd

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"time"

	"github.com/xmx/aegis-common/library/timex"
	"github.com/xmx/aegis-common/muxlink/muxconn"
	"github.com/xmx/aegis-common/muxlink/muxproto"
)

func Open(cfg muxconn.DialConfig, opt Options) (muxconn.Muxer, error) {
	opt = opt.format()
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}

	mux := new(muxInstance)
	ac := &agentClient{cfg: cfg, opt: opt, mux: mux}
	if err := ac.opens(); err != nil {
		return nil, err
	}

	go ac.serve(mux)

	return mux, nil
}

type agentClient struct {
	cfg     muxconn.DialConfig
	opt     Options
	mux     *muxInstance
	rebuild bool
}

func (ac *agentClient) Muxer() muxconn.Muxer {
	return ac.mux
}

func (ac *agentClient) opens() error {
	ac.log().Info("准备连接 broker 服务", "addresses", ac.cfg.Addresses)
	machineID := ac.machineID(false)
	req := &authRequest{
		MachineID: machineID,
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

	timeout := ac.cfg.PerTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var fails int
	for {
		mux, res, err := ac.open(req, timeout)
		if err == nil {
			ac.mux.store(mux)
			return nil
		}

		fails++
		wait := ac.waitTime(fails)
		ac.log().Warn("等待重连", "fails", fails, "sleep", wait, "error", err)
		if err = timex.Sleep(ac.cfg.Context, wait); err != nil {
			ac.log().Error("不满足继续重试连接的条件", "error", err)
			return err
		}

		if res == nil || !res.conflicted() || ac.rebuild {
			continue
		}

		// 检测到该机器码已经是在线状态，尝试重新生产机器码。
		ac.rebuild = true
		beforeID := req.MachineID
		afterID := ac.machineID(true)
		if beforeID == afterID {
			ac.log().Info("生成的机器码前后一致", "machine_id", beforeID)
			continue
		}

		req.MachineID = afterID
		ac.log().Warn("重新生成了新的机器码", "before_id", beforeID, "after_id", afterID)
	}
}

func (ac *agentClient) open(req *authRequest, timeout time.Duration) (muxconn.Muxer, *authResponse, error) {
	attrs := []any{slog.Any("addresses", ac.cfg.Addresses)}
	mux, err := muxconn.Open(ac.cfg)
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
		ac.log().Warn("基础网络连接失败", attrs...)
		return nil, nil, err
	}

	laddr, raddr := mux.Addr(), mux.RemoteAddr()
	outboundIP := muxproto.Outbound(laddr, raddr)
	req.Inet = outboundIP.String()
	protocol, subprotocol := mux.Protocol()
	attrs = append(attrs,
		slog.Any("protocol", protocol),
		slog.Any("subprotocol", subprotocol),
		slog.Any("local_addr", laddr),
		slog.Any("remote_addr", raddr),
		slog.Any("outbound_ip", outboundIP),
	)

	ac.log().Info("基础网络连接成功，开始交换认证报文", attrs...)
	res, err1 := ac.authentication(mux, req, timeout)
	if err1 != nil {
		_ = mux.Close()
		attrs = append(attrs, slog.Any("error", err1))
		ac.log().Warn("认证报文交换错误", attrs...)
		return nil, nil, err1
	}

	attrs = append(attrs, slog.Any("auth_response", res))
	if err = res.checkError(); err != nil {
		_ = mux.Close()
		attrs = append(attrs, slog.Any("error", err))
		ac.log().Warn("基础网络连接成功但认证未通过", attrs...)
		return nil, res, err
	}
	ac.log().Info("通道连接认证成功", attrs...)

	return mux, res, nil
}

//goland:noinspection GoUnhandledErrorResult
func (ac *agentClient) authentication(mux muxconn.Muxer, req *authRequest, timeout time.Duration) (*authResponse, error) {
	ctx, cancel := context.WithTimeout(ac.cfg.Context, timeout)
	defer cancel()

	conn, err := mux.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	if err = muxproto.WriteJSON(conn, req); err != nil {
		return nil, err
	}

	resp := new(authResponse)
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	if err = muxproto.ReadJSON(conn, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (ac *agentClient) serve(mux muxconn.Muxer) {
	for {
		srv := ac.server()
		err := srv.Serve(mux)
		_ = mux.Close()

		ac.log().Warn("通道掉线了", "error", err)
		if err = ac.opens(); err != nil {
			break
		}
	}
}

func (ac *agentClient) server() *http.Server {
	h := ac.opt.Handler
	if h == nil {
		h = http.NotFoundHandler()
	}

	return &http.Server{Handler: h}
}

func (ac *agentClient) machineID(rebuild bool) string {
	return ac.opt.Ident.MachineID(rebuild)
}

// waitTime 通过持续连接耗费的时间和次数，计算出一个合理的重试时间。
func (ac *agentClient) waitTime(fails int) time.Duration {
	if fails <= 100 {
		return 3 * time.Second
	} else if fails <= 300 {
		return 10 * time.Second
	} else if fails <= 500 {
		return 30 * time.Second
	}

	return time.Minute
}

func (ac *agentClient) log() *slog.Logger {
	if l := ac.opt.Logger; l != nil {
		return l
	}

	return slog.Default()
}
