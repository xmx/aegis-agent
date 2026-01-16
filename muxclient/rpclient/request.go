package rpclient

type requestData struct {
	Data any `json:"data"`
}

type NetworkCard struct {
	Name  string   `json:"name"`
	Index int      `json:"index"`
	MTU   int      `json:"mtu"`
	IPv4  []string `json:"ipv4"`
	IPv6  []string `json:"ipv6"`
	MAC   string   `json:"mac"`
}

type NetworkCards []*NetworkCard
