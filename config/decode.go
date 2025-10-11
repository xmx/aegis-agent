package config

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"

	"github.com/xmx/aegis-common/jsos/jsmod"
	"github.com/xmx/aegis-common/jsos/jsvm"
	"github.com/xmx/aegis-common/library/jsonc"
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
	varb := jsmod.NewVariable[HideConfig]("aegis/agent/config")
	varb.Set(cfg)
	require := vm.Require()
	require.Registers(jsmod.Modules())
	require.Register(varb)

	if _, err = vm.RunScript(filename, string(data)); err != nil {
		return nil, err
	}

	return varb.Get(), nil
}

type jsoncdecoder struct{}

func (j jsoncdecoder) decode(_ context.Context, filename string) (*HideConfig, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	data := jsonc.Translate(raw)
	cfg := new(HideConfig)
	if err = json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
