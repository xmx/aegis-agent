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
	"github.com/xmx/aegis-agent/clientd"
	"github.com/xmx/aegis-agent/config"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/library/httpx"
	"github.com/xmx/aegis-common/logger"
	"github.com/xmx/aegis-common/shipx"
	"github.com/xmx/aegis-common/tunnel/tundial"
	"github.com/xmx/aegis-common/tunnel/tunutil"
	"github.com/xmx/aegis-common/validation"
)

func Exec(ctx context.Context, configFile string) error {
	consoleOut := logger.NewTint(os.Stdout)
	logHandler := logger.NewHandler(consoleOut)
	log := slog.New(logHandler)

	configLoader := config.File(configFile)
	cfg, err := configLoader.Load(ctx)
	if err != nil {
		log.Error("加载配置文件错误", "error", err)
	}
	if cfg == nil {
		cfg = &config.HideConfig{}
	}

	valid := validation.New()
	brkHandler := httpx.NewAtomicHandler(nil)
	tunCfg := tundial.Config{
		Protocols:  cfg.Protocols,
		Addresses:  cfg.Addresses,
		PerTimeout: 10 * time.Second,
		Parent:     ctx,
	}
	cliOpt := clientd.NewOption().Handler(brkHandler).Logger(log)
	tunnel, err := clientd.Open(tunCfg, cliOpt)
	if err != nil {
		return err
	}
	systemDialer := tunutil.DefaultDialer()         // 系统默认 dialer
	tunnelDialer := tunutil.NewTunnelDialer(tunnel) // 将 tunnel 改造成 dialer

	dialer := tunutil.NewMatchDialer(systemDialer, tunutil.NewHostMatch(tunutil.BrokerHost, tunnelDialer))
	httpTransport := &http.Transport{DialContext: dialer.DialContext}
	httpCli := httpx.NewClient(&http.Client{Transport: httpTransport})
	crond := cronv3.New(ctx, log, cron.WithSeconds())
	crond.Start()

	networkTask := crontab.NewNetwork(ctx, httpCli, log)
	_, _ = crond.AddTask(networkTask)

	brokerAPIs := []shipx.RouteRegister{
		shipx.NewPprof(),
		shipx.NewHealth(),
		restapi.NewSystem(tunnel),
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
	_ = tunnel.Close()

	return ctx.Err()
}
