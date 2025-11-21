package clientd

import (
	"fmt"
	"log/slog"
	"net/http"
)

type authRequest struct {
	MachineID  string   `json:"machine_id"`
	Inet       string   `json:"inet"`
	Goos       string   `json:"goos"`
	Goarch     string   `json:"goarch"`
	PID        int      `json:"pid,omitzero"`
	Args       []string `json:"args,omitzero"`
	Hostname   string   `json:"hostname,omitzero"`
	Workdir    string   `json:"workdir,omitzero"`
	Executable string   `json:"executable,omitzero"`
	Username   string   `json:"username,omitzero"`
}

type authResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitzero"`
}

func (ar authResponse) LogValue() slog.Value {
	if err := ar.checkError(); err != nil {
		return slog.StringValue(err.Error())
	}

	return slog.StringValue("认证接入成功")
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
