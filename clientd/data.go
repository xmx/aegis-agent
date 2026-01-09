package clientd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/xmx/aegis-common/muxlink/muxconn"
)

// Identifier agent 身份唯一标识。
type Identifier interface {
	// MachineID 获取机器码，机器码是 agent 节点的唯一标识。
	//
	// 在实际环境中，业务方的会进行动态扩缩容，他们会用基础镜像克隆出多个实例，这个基础镜像可能
	// 已经运行过 ssoc-agent，机器码和环境已经初始化过了，而 agent 自身并不知道自己是克隆体，
	// 但是 ssoc 服务就会任务节点在重复连接而拒绝上线。
	//
	// 针对上述问题，设计一种机器码冲突避让策略：
	// 生成机器码时一定要根据操作系统环境生成，不能用时间戳、UUID 等随机的参数作为机器码。
	// 例如：使用计算操作系统的 machine-id + hostname + mac + ip 哈希值作为机器码。即便是
	// 镜像克隆出来的机器，它们的 machine-id 一样，但是 hostname mac ip 不太可能一样，因为
	// 扩缩容的机器一般都同处一个局域网，如果 mac ip 一样，这台机器大概率无法正常联网工作的。
	//
	// 虽然有稳定的生成算法，生成机器码生成后要保存在本地磁盘，如果没有指定要 rebuild 可以直接
	// 读取本地缓存的机器码，为什么要保存在本地呢？因为 agent 的 ip 可能是 DHCP，hostname 也
	// 可能被修改，但机器还是那台机器，如果不缓存每次都生成，久而久之会导致服务端留存大量无效节点。
	//
	// 总结大致思路就是：每次上线时如果服务端检测到重复连接，agent 就 rebuild 重新生成机器码，
	// 每次上线时 rebuild 至多会被调用一次，rebuild 后的机器码可能还是原来的机器码，这说明
	// agent 环境没有发生变化。如果 rebuild 后还是重复上线，也不会再次 rebuild，一般说明存
	// 在其它问题。
	MachineID(rebuild bool) string
}

type authRequest struct {
	MachineID  string   `json:"machine_id"`
	Semver     string   `json:"semver"`
	Inet       string   `json:"inet"`
	Goos       string   `json:"goos"`
	Goarch     string   `json:"goarch"`
	PID        int      `json:"pid,omitzero"`
	Args       []string `json:"args,omitzero"`
	Hostname   string   `json:"hostname,omitzero"`
	Workdir    string   `json:"workdir,omitzero"`
	Executable string   `json:"executable,omitzero"`
}

type authResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitzero"`
}

func (ar authResponse) String() string {
	if err := ar.checkError(); err != nil {
		return err.Error()
	}

	return "认证接入成功"
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
