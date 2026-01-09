package clientd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/xmx/aegis-common/muxlink/muxconn"
)

type authRequest struct {
	MachineID  string   `json:"machine_id"`
	Inet       string   `json:"inet"`
	Goos       string   `json:"goos"`
	Goarch     string   `json:"goarch"`
	PID        int      `json:"pid,omitzero"`
	Args       []string `json:"args,omitzero"`
	Hostname   string   `json:"hostname,omitzero"`
	Workdir    string   `json:"workdir,omitzero"`
	Executable string   `json:"executable,omitzero"`
	Username   string   `json:"username,omitzero"`
}

type authResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitzero"`
}

func (ar authResponse) LogValue() slog.Value {
	if err := ar.checkError(); err != nil {
		return slog.StringValue(err.Error())
	}

	return slog.StringValue("认证接入成功")
}

func (ar authResponse) checkError() error {
	code := ar.Code
	if code >= http.StatusOK && code < http.StatusMultipleChoices {
		return nil
	}

	return fmt.Errorf("agent 认证失败 %d: %s", ar.Code, ar.Message)
}

func (ar authResponse) conflicted() bool {
	return ar.Code == http.StatusConflict
}

type muxInstance struct {
	ptr atomic.Pointer[muxconn.Muxer]
}

func (m *muxInstance) Accept() (net.Conn, error)                  { return m.load().Accept() }
func (m *muxInstance) Close() error                               { return m.load().Close() }
func (m *muxInstance) Addr() net.Addr                             { return m.load().Addr() }
func (m *muxInstance) Open(ctx context.Context) (net.Conn, error) { return m.load().Open(ctx) }
func (m *muxInstance) RemoteAddr() net.Addr                       { return m.load().RemoteAddr() }
func (m *muxInstance) Protocol() (string, string)                 { return m.load().Protocol() }
func (m *muxInstance) Traffic() (rx, tx uint64)                   { return m.load().Traffic() }
func (m *muxInstance) load() muxconn.Muxer                        { return *m.ptr.Load() }
func (m *muxInstance) store(mux muxconn.Muxer)                    { m.ptr.Store(&mux) }
