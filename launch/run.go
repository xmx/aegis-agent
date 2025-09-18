package launch

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/applet/restapi"
	"github.com/xmx/aegis-agent/client/tunnel"
	"github.com/xmx/aegis-agent/config"
	"github.com/xmx/aegis-agent/machine"
	"github.com/xmx/aegis-common/library/httpx"
	"github.com/xmx/aegis-common/logger"
	"github.com/xmx/aegis-common/shipx"
	"github.com/xmx/aegis-common/transport"
	"github.com/xmx/aegis-common/validation"
)

func Exec(ctx context.Context, ld config.Loader) error {
	consoleOut := logger.NewTint(os.Stdout)
	logHandler := logger.NewHandler(consoleOut)
	log := slog.New(logHandler)

	cfg := &config.Config{Addresses: []string{"127.0.0.1:9443"}}

	valid := validation.New()
	if err := valid.Validate(cfg); err != nil {
		log.Error("配置验证错误", slog.Any("error", err))
		return err
	}

	machineID := machine.HashedID()
	brkHandler := httpx.NewAtomicHandler(nil)
	dc := tunnel.DialConfig{
		MachineID: machineID,
		Addresses: cfg.Addresses,
		Handler:   brkHandler,
		DialConfig: transport.DialConfig{
			Parent: ctx,
		},
		Timeout: 10 * time.Second,
		Logger:  log,
	}
	cli, err := dc.Open()
	if err != nil {
		return err
	}

	brokerAPIs := []shipx.RouteRegister{
		restapi.NewPprof(),
		restapi.NewSystem(cli),
	}
	shipLog := logger.NewShip(logHandler, 6)
	brkSH := ship.Default()
	brkSH.NotFound = shipx.NotFound
	brkSH.HandleError = shipx.HandleError
	brkSH.Validator = valid
	brkSH.Logger = shipLog
	brkHandler.Store(brkSH)

	apiRGB := brkSH.Group("/api")
	if err = shipx.RegisterRoutes(apiRGB, brokerAPIs); err != nil {
		return err
	}

	<-ctx.Done()

	return ctx.Err()
}
