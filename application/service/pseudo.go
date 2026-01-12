package service

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/xmx/aegis-agent/application/request"
	"github.com/xmx/aegis-common/wsocket"
)

func NewPseudo(log *slog.Logger) *Pseudo {
	return &Pseudo{
		log: log,
	}
}

type Pseudo struct {
	log *slog.Logger
}

//goland:noinspection GoUnhandledErrorResult
func (psd *Pseudo) TTY(ws *websocket.Conn, req *request.PseudoSize) error {
	bash := os.Getenv("SHELL")
	if bash == "" {
		bash = "sh"
	}

	psd.log.Info("准备启动虚拟终端", "bash", bash)
	stdout := wsocket.NewTTYWriter(ws, "stdout")
	stderr := wsocket.NewTTYWriter(ws, "stderr")
	pong := wsocket.NewTTYWriter(ws, "pong")
	cmd := exec.Command(bash)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	var err error
	var ptmx *os.File
	if req == nil || req.IsZero() {
		ptmx, err = pty.Start(cmd)
	} else {
		ptmx, err = pty.StartWithSize(cmd, req.Winsize())
	}
	if err != nil {
		psd.log.Error("启动虚拟终端错误", "bash", bash, "error", err)
		_ = wsocket.CloseControl(ws, err)
		return err
	}
	defer ptmx.Close()

Read:
	for {
		attrs := []any{"bash", bash}
		msg := new(wsocket.TypeMessage)
		if err = ws.ReadJSON(msg); err != nil {
			attrs = append(attrs, "error", err)
			psd.log.Warn("读取输入消息出错", attrs...)
			return err
		}

		msgType := strings.ToLower(msg.Type)
		attrs = append(attrs, "msg_type", msgType)
		switch msgType {
		case "stdin":
			var data string
			if err = msg.Unmarshal(&data); err != nil {
				psd.log.Error("反序列化消息出错", attrs...)
				return err
			}

			attrs = append(attrs, "stdin", data)
			if _, err = ptmx.WriteString(data); err != nil {
				attrs = append(attrs, "error", err)
				psd.log.Warn("写入虚拟终端出错", attrs...)
				return err
			}

			psd.log.Debug("虚拟终端写入消息", attrs...)
		case "resize":
			data := new(request.PseudoSize)
			if err = msg.Unmarshal(data); err != nil {
				psd.log.Error("反序列化消息出错", attrs...)
				return err
			}
			if data.IsZero() {
				continue
			}

			attrs = append(attrs, "size", data)
			if err = pty.Setsize(ptmx, data.Winsize()); err != nil {
				attrs = append(attrs, "error", err)
				psd.log.Warn("修改虚拟终端窗口大小出错", attrs...)
				return err
			}

			psd.log.Debug("修改虚拟终端窗口大小", attrs...)
		case "ping":
			dt, _ := time.Now().MarshalText()
			_, _ = pong.Write(dt)
			psd.log.Debug("接收到心跳消息", attrs...)
		default:
			psd.log.Info("接收到不支持的消息类型", attrs...)
			err = errors.ErrUnsupported
			_ = wsocket.CloseControl(ws, err)
			break Read
		}
	}

	return err
}
