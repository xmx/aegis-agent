package config

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
)

type Loader interface {
	Load(context.Context) (*Config, error)
}

type Config struct {
	Protocols []string `json:"protocols"`
	Addresses []string `json:"addresses"`
}

type JSON string

func (j JSON) Load(context.Context) (*Config, error) {
	f, err := os.Open(string(j))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := new(Config)
	dec := jsontext.NewDecoder(f)
	if err = json.UnmarshalDecode(dec, c); err != nil {
		return nil, err
	}

	return c, nil
}
