package launch

import (
	"context"
	"math/rand/v2"
	"net"
	"net/netip"
)

// FIXME 临时性补丁，此方式修改了全局的 DNS 逻辑。
func init() {
	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, exx := net.SplitHostPort(addr)
		if exx != nil {
			return nil, exx
		}

		if ip, _ := netip.ParseAddr(host); ip.IsLoopback() {
			servers := []string{
				"233.5.5.5:53", "114.114.114.114:53", "180.76.76.76:53",
				"1.2.4.8:53", "8.8.8.8:53", "119.29.29.29:53",
			}
			idx := rand.IntN(len(servers))
			addr = servers[idx]
		}

		var d net.Dialer

		return d.DialContext(ctx, network, addr)
	}
}
