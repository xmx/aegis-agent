package launch

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/applet/crontab"
	"github.com/xmx/aegis-agent/applet/restapi"
	"github.com/xmx/aegis-agent/client/tunnel"
	"github.com/xmx/aegis-agent/config"
	"github.com/xmx/aegis-agent/machine"
	"github.com/xmx/aegis-common/library/cronv3"
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

	cfg, err := ld.Load(ctx)
	if err != nil {
		log.Error("加载配置文件错误", "error", err)
	}
	if cfg == nil {
		cfg = &config.HideConfig{}
	}

	valid := validation.New()
	machineID := machine.HashedID()
	brkHandler := httpx.NewAtomicHandler(nil)
	dc := tunnel.DialConfig{
		MachineID: machineID,
		Addresses: cfg.Addresses,
		Handler:   brkHandler,
		DialConfig: transport.DialConfig{
			Protocols: cfg.Protocols,
			Parent:    ctx,
		},
		Timeout: 10 * time.Second,
		Logger:  log,
	}
	cli, err := dc.Open()
	if err != nil {
		return err
	}

	trip := transport.NewHTTPTransport(cli.MuxLoader(), transport.BrokerHost)
	httpCli := httpx.Client{Client: &http.Client{Transport: trip}}
	crond := cronv3.New(ctx, log, cron.WithSeconds())
	crond.Start()

	networkTask := crontab.NewNetwork(ctx, httpCli, log)
	_, _ = crond.AddTask(networkTask)

	brokerAPIs := []shipx.RouteRegister{
		shipx.NewPprof(),
		shipx.NewHealth(),
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
	crond.Stop()
	_ = cli.Close()

	return ctx.Err()
}
