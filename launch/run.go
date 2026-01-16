package launch

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/grafana/pyroscope-go"
	"github.com/robfig/cron/v3"
	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/application/crontab"
	"github.com/xmx/aegis-agent/application/restapi"
	"github.com/xmx/aegis-agent/application/service"
	"github.com/xmx/aegis-agent/config"
	"github.com/xmx/aegis-agent/muxclient/clientd"
	"github.com/xmx/aegis-agent/muxclient/rpclient"
	"github.com/xmx/aegis-common/banner"
	jscron "github.com/xmx/aegis-common/jsos/jslib/cron"
	"github.com/xmx/aegis-common/jsos/jsstd"
	"github.com/xmx/aegis-common/jsos/jstask"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/library/validation"
	"github.com/xmx/aegis-common/logger"
	"github.com/xmx/aegis-common/muxlink/muxconn"
	"github.com/xmx/aegis-common/muxlink/muxproto"
	"github.com/xmx/aegis-common/profile"
	"github.com/xmx/aegis-common/shipx"
	"github.com/xmx/aegis-common/stegano"
)

func Run(ctx context.Context, cfg string) error {
	var cfr profile.Reader[config.Config]
	if cfg != "" {
		cfr = profile.File[config.Config](cfg)
	} else {
		exe := os.Args[0]
		cfr = stegano.File[config.Config](exe)
	}

	return Exec(ctx, cfr)
}

//goland:noinspection GoUnhandledErrorResult
func Exec(ctx context.Context, crd profile.Reader[config.Config]) error {
	logOpts := &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}
	logh := logger.NewMultiHandler(logger.NewTint(os.Stdout, logOpts))
	log := slog.New(logh)

	// 即便配置文件加载错误，尽量使用默认值启动。
	cfg, err := crd.Read()
	if err != nil {
		log.Error("加载配置文件错误", "error", err)
	}
	if cfg == nil {
		cfg = new(config.Config)
	}

	valid := validation.New()
	shipLog := logger.NewShip(logh)
	brkSH := ship.Default()
	brkSH.NotFound = shipx.NotFound
	brkSH.HandleError = shipx.HandleError
	brkSH.Validator = valid
	brkSH.Logger = shipLog

	tunCfg := muxconn.DialConfig{
		Protocols:  cfg.Protocols,
		Addresses:  cfg.Addresses,
		PerTimeout: 10 * time.Second,
		Logger:     log,
		Context:    ctx,
	}
	buildInfo := banner.SelfInfo()
	tunCliOpts := clientd.Options{Semver: buildInfo.Semver, Handler: brkSH}

	mux, err := clientd.Open(tunCfg, tunCliOpts)
	if err != nil {
		return err
	}

	netDialer := &net.Dialer{Timeout: 30 * time.Second}
	tunDialer := muxproto.NewMatchHostDialer(muxproto.BrokerHost, mux)
	dualDialer := muxproto.NewFirstMatchDialer([]muxproto.Dialer{tunDialer}, netDialer)
	rpcli := rpclient.NewClient(dualDialer, log)

	prof, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "aegis-agent",
		ServerAddress:   muxproto.AgentToBrokerURL("/api/pyroscope").String(),
		HTTPClient:      rpcli,
		ProfileTypes: []pyroscope.ProfileType{ // 常用 profile 类型（按需开）
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		log.Error("pyroscope 启动错误", "error", err)
	} else {
		defer prof.Stop()
	}

	parserOpts := cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor
	crond := cronv3.New(ctx, log, cron.WithParser(cron.NewParser(parserOpts)))
	crond.Start()

	cronTasks := []cronv3.Tasker{
		crontab.NewHealth(rpcli),
		crontab.NewNetwork(rpcli),
		crontab.NewMetrics(rpcli),
	}
	for _, task := range cronTasks {
		_, _ = crond.AddTask(task)
	}

	taskOpt := jstask.Options{
		Logger:  log,
		Stdout:  []io.Writer{os.Stdout},
		Stderr:  []io.Writer{os.Stderr},
		Module:  jsstd.All(),
		Context: ctx,
	}
	taskOpt.Module = append(taskOpt.Module, jscron.New())

	jsManager := jstask.New(taskOpt)
	taskSvc := service.NewTask(jsManager, log)
	systemSvc := service.NewSystem(log)

	brokerAPIs := []shipx.RouteRegister{
		shipx.NewPprof(),
		shipx.NewHealth(),
		restapi.NewSystem(systemSvc),
		restapi.NewEcho(),
		restapi.NewTask(taskSvc),
	}
	apiRGB := brkSH.Group("/api")
	if err = shipx.RegisterRoutes(apiRGB, brokerAPIs); err != nil {
		return err
	}

	<-ctx.Done()
	crond.Stop()
	_ = mux.Close()
	rx, tx := mux.Traffic()
	log.Info("流量统计", "receive_bytes", rx, "transmit_bytes", tx)

	return ctx.Err()
}
