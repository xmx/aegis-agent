package clientd

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"net"
	"os"
	"slices"
	"strings"

	"github.com/xmx/aegis-agent/machine"
)

func NewIdent(storeFile string) Identifier {
	return &machineIdent{
		f: storeFile,
	}
}

type machineIdent struct {
	f string
}

func (m *machineIdent) MachineID(rebuild bool) string {
	if !rebuild { // 从缓存读取
		if id := m.raed(); id != "" {
			return id
		}
	}

	id := m.generate()
	if id == "" {
		buf := make([]byte, 20)
		_, _ = rand.Read(buf)
		id = hex.EncodeToString(buf)
	}
	m.store(id)

	return id
}

func (m *machineIdent) raed() string {
	if m.f == "" {
		return ""
	}

	b, err := os.ReadFile(m.f)
	if err != nil {
		return ""
	}

	return strings.TrimSuffix(string(b), "\n")
}

func (m *machineIdent) store(id string) {
	if m.f != "" {
		_ = os.WriteFile(m.f, []byte(id), 0600)
	}
}

func (m *machineIdent) generate() string {
	mid, _ := machine.ID()
	hostname, _ := os.Hostname()
	card := m.networks()
	str := strings.Join([]string{mid, hostname, card}, ",")
	sum := sha1.Sum([]byte(str))

	return hex.EncodeToString(sum[:])
}

func (*machineIdent) networks() string {
	faces, _ := net.Interfaces()
	cards := make(nics, 0, len(faces))
	for _, face := range faces {
		// 跳过换回网卡和未启用的网卡
		if face.Flags&net.FlagUp == 0 ||
			face.Flags&net.FlagLoopback != 0 {
			continue
		}

		var ips []string
		addrs, _ := face.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}

			// 过滤无效地址
			if ip == nil ||
				ip.IsLoopback() ||
				ip.IsMulticast() ||
				ip.IsUnspecified() {
				continue
			}

			if ip4 := ip.To4(); ip4 != nil {
				ips = append(ips, ip.String())
			} else if ip.To16() != nil {
				// 排除 IPv6 链路本地地址（fe80::/10），如不需要可移除此条件
				if ip.IsLinkLocalUnicast() {
					continue
				}
				ips = append(ips, ip.String())
			}
			if len(ips) != 0 {
				cards = append(cards, &nic{
					MAC:   face.HardwareAddr.String(),
					Inets: ips,
				})
			}
		}
	}
	cards.sort()

	return cards.join()
}

type nic struct {
	MAC   string
	Inets []string
}

type nics []*nic

func (ns nics) sort() {
	slices.SortFunc(ns, func(a, b *nic) int {
		return strings.Compare(a.MAC, b.MAC)
	})
	for _, n := range ns {
		slices.Sort(n.Inets)
	}
}

func (ns nics) join() string {
	strs := make([]string, 0, len(ns))
	for _, n := range ns {
		ele := make([]string, 0, len(n.Inets)+1)
		ele = append(ele, n.MAC)
		ele = append(ele, n.Inets...)
		line := strings.Join(ele, ",")
		strs = append(strs, line)
	}

	return strings.Join(strs, ",")
}
