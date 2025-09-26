package config

import (
	"context"
	"path/filepath"
	"strings"
)

type Loader interface {
	Load(context.Context) (*HideConfig, error)
}

func NewFile(f string) Loader {
	ld := &loader{filename: f}
	switch strings.ToLower(filepath.Ext(f)) {
	case ".js":
		ld.decoder = new(jsdecoder)
	case ".json":
		ld.decoder = new(jsondecoder)
	default:
		ld.decoder = new(undecoder)
	}

	return ld
}

type HideConfig struct {
	Protocols []string `json:"protocols"` // 连接协议 udp tcp
	Addresses []string `json:"addresses"` // broker 地址
}

type loader struct {
	filename string
	decoder  decoder
}

func (l loader) Load(ctx context.Context) (*HideConfig, error) {
	return l.decoder.decode(ctx, l.filename)
}

type decoder interface {
	decode(ctx context.Context, filename string) (*HideConfig, error)
}
