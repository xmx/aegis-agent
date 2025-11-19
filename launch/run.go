package launch

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/netip"
	"os"
	"runtime"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xgfone/ship/v5"
	"github.com/xmx/aegis-agent/application/crontab"
	"github.com/xmx/aegis-agent/application/restapi"
	"github.com/xmx/aegis-agent/application/service"
	"github.com/xmx/aegis-agent/clientd"
	"github.com/xmx/aegis-agent/config"
	"github.com/xmx/aegis-common/jsos/jsmod"
	"github.com/xmx/aegis-common/jsos/jstask"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/library/httpkit"
	"github.com/xmx/aegis-common/library/validation"
	"github.com/xmx/aegis-common/logger"
	"github.com/xmx/aegis-common/profile"
	"github.com/xmx/aegis-common/shipx"
	"github.com/xmx/aegis-common/stegano"
	"github.com/xmx/aegis-common/tunnel/tundial"
	"github.com/xmx/aegis-common/tunnel/tunutil"
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

func Exec(ctx context.Context, crd profile.Reader[config.Config]) error {
	logh := logger.NewHandler(logger.NewTint(os.Stdout, nil))
	log := slog.New(logh)

	// 即便配置文件加载错误，尽量使用默认值启动。
	cfg, err := crd.Read()
	if err != nil {
		log.Error("加载配置文件错误", "error", err)
	}
	if cfg == nil {
		cfg = new(config.Config)
	}

	// FIXME 临时性补丁，此方式修改了全局的 DNS 逻辑。
	if runtime.GOOS == "android" {
		net.DefaultResolver.PreferGo = true
		net.DefaultResolver.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, exx := net.SplitHostPort(addr)
			if exx != nil {
				return nil, exx
			}

			if ip, _ := netip.ParseAddr(host); ip.IsLoopback() {
				servers := []string{
					"233.5.5.5:53", "114.114.114.114:53", "180.76.76.76:53",
					"1.2.4.8:53", "8.8.8.8:53", "119.29.29.29:53",
				}
				idx := rand.IntN(len(servers))
				addr = servers[idx]
			}
			log.Info("请求 DNS 服务器", "server", addr)

			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}
	}

	valid := validation.New()
	brkHandler := httpkit.NewHandler()
	tunCfg := tundial.Config{
		Protocols:  cfg.Protocols,
		Addresses:  cfg.Addresses,
		PerTimeout: 10 * time.Second,
		Parent:     ctx,
	}
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

	modules := jsmod.All()
	modules = append(modules, jsmod.NewCrontab(crond))
	jstOpt := jstask.Option{Modules: modules, Stdout: os.Stdout, Stderr: os.Stdout}
	jsManager := jstask.NewManager(jstOpt)
	taskSvc := service.NewTask(jsManager, log)

	brokerAPIs := []shipx.RouteRegister{
		shipx.NewPprof(),
		shipx.NewHealth(),
		restapi.NewSystem(),
		restapi.NewEcho(),
		restapi.NewTask(taskSvc),
	}
	shipLog := logger.NewShip(logh)
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
