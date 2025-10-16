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
	"github.com/xmx/aegis-agent/applet/service"
	"github.com/xmx/aegis-agent/clientd"
	"github.com/xmx/aegis-agent/config"
	"github.com/xmx/aegis-common/jsos/jsexec"
	"github.com/xmx/aegis-common/jsos/jsmod"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/library/httpkit"
	"github.com/xmx/aegis-common/library/validation"
	"github.com/xmx/aegis-common/logger"
	"github.com/xmx/aegis-common/profile"
	"github.com/xmx/aegis-common/shipx"
	"github.com/xmx/aegis-common/tunnel/tundial"
	"github.com/xmx/aegis-common/tunnel/tunutil"
)

func Run(ctx context.Context, cfg string) error {
	// 2<<22 = 8388608 (8 MiB)
	opt := profile.NewOption().Limit(2 << 22).ModuleName("aegis/agent/config")
	crd := profile.NewFile[config.Config](cfg, opt)

	return Exec(ctx, crd)
}

func Exec(ctx context.Context, crd profile.Reader[config.Config]) error {
	logHandler := logger.NewHandler(logger.NewTint(os.Stdout))
	log := slog.New(logHandler)

	// 即便配置文件加载错误，尽量使用默认值启动。
	cfg, err := crd.Read(ctx)
	if err != nil {
		log.Error("加载配置文件错误", "error", err)
	}
	if cfg == nil {
		cfg = new(config.Config)
	}

	valid := validation.New()
	brkHandler := httpkit.NewAtomicHandler(nil)
	tunCfg := tundial.Config{
		Protocols:  cfg.Protocols,
		Addresses:  cfg.Addresses,
		PerTimeout: 10 * time.Second,
		Parent:     ctx,
	}
	log.Info("开始连接 broker 服务器")
	cliOpt := clientd.NewOption().Handler(brkHandler).Logger(log)
	mux, err := clientd.Open(tunCfg, cliOpt)
	if err != nil {
		return err
	}
	defaultDialer := tunutil.DefaultDialer() // 系统默认 dialer
	muxDialer := tunutil.NewMuxDialer(mux)   // 将 tunnel 改造成 dialer
	brokerDialer := tunutil.NewHostMatchDialer(tunutil.BrokerHost, muxDialer)
	dialer := tunutil.NewMatchDialer(defaultDialer, brokerDialer)
	httpTransport := &http.Transport{DialContext: dialer.DialContext}
	httpCli := httpkit.NewClient(&http.Client{Transport: httpTransport})

	crond := cronv3.New(ctx, log, cron.WithSeconds())
	crond.Start()

	cronTasks := []cronv3.Tasker{
		crontab.NewNetwork(httpCli),
	}
	for _, task := range cronTasks {
		_, _ = crond.AddTask(task)
	}

	modules := jsmod.Modules()
	modules = append(modules, jsmod.NewCrontab(crond))
	jsmOpt := jsexec.NewOption().Modules(modules)
	jsManager := jsexec.NewManager(jsmOpt)
	taskSvc := service.NewTask(jsManager, log)

	brokerAPIs := []shipx.RouteRegister{
		shipx.NewPprof(),
		shipx.NewHealth(),
		restapi.NewSystem(),
		restapi.NewEcho(),
		restapi.NewTask(taskSvc),
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
	_ = mux.Close()
	rx, tx := mux.Transferred()
	log.Info("流量统计", "receive_bytes", rx, "transmit_bytes", tx)

	return ctx.Err()
}
