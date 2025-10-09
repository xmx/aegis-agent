package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

type HideConfig struct {
	Protocols []string `json:"protocols"` // 连接协议 udp tcp
	Addresses []string `json:"addresses"` // broker 地址
}

type Loader interface {
	Load(context.Context) (*HideConfig, error)
}

type File string

func (f File) Load(ctx context.Context) (*HideConfig, error) {
	name := filepath.Join("./", string(f))
	stat, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return f.readDir(ctx, name)
	}

	return f.readFile(ctx, name)
}

func (f File) readFile(ctx context.Context, filename string) (*HideConfig, error) {
	ext := filepath.Ext(filename)
	switch ext {
	case ".js":
		dec := new(jsdecoder)
		return dec.decode(ctx, filename)
	case ".json":
		dec := new(jsondecoder)
		return dec.decode(ctx, filename)
	case ".jsonc":
		dec := new(jsoncdecoder)
		return dec.decode(ctx, filename)
	default:
		return nil, errors.ErrUnsupported
	}
}

func (f File) readDir(ctx context.Context, basedir string) (*HideConfig, error) {
	basename := "application"

	// 按照 .js .jsonc .json 顺序读取配置文件，直到第一个正确的停止。
	errs := make([]error, 0, 3)
	names := []string{basename + ".js", basename + ".jsonc", basename + ".json"}
	for _, name := range names {
		cfg, err := f.readFile(ctx, filepath.Join(basedir, name))
		if err == nil {
			return cfg, nil
		}
		errs = append(errs, err)
	}

	return nil, errors.Join(errs...)
}
