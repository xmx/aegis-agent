package config

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"os"

	"github.com/xmx/aegis-common/jsos/jsmod"
	"github.com/xmx/aegis-common/jsos/jsvm"
)

type jsondecoder struct{}

func (j jsondecoder) decode(_ context.Context, filename string) (*HideConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := new(HideConfig)
	dec := jsontext.NewDecoder(f)
	if err = json.UnmarshalDecode(dec, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

type jsdecoder struct{}

func (j jsdecoder) decode(ctx context.Context, filename string) (*HideConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	vm := jsvm.New(ctx)
	stdout, stderr := vm.Output()
	stdout.Attach(os.Stdout)
	stderr.Attach(os.Stderr)

	cfg := new(HideConfig)
	varb := jsmod.NewVariable[*HideConfig]("aegis/agent/config")
	varb.Set(cfg)
	require := vm.Require()
	require.Registers(jsmod.Modules())
	require.Register(varb)

	if _, err = vm.RunScript(filename, string(data)); err != nil {
		return nil, err
	}

	return varb.Get(), nil
}

type undecoder struct{}

func (u undecoder) decode(context.Context, string) (*HideConfig, error) {
	return nil, errors.ErrUnsupported
}
