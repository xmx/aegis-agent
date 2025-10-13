package config

type Config struct {
	Protocols []string `json:"protocols"` // 连接协议 udp tcp
	Addresses []string `json:"addresses"` // broker 地址
}
