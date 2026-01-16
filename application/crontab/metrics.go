package crontab

import (
	"context"
	"io"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xmx/aegis-agent/muxclient/rpclient"
	"github.com/xmx/aegis-common/library/cronv3"
	"github.com/xmx/aegis-common/muxlink/muxproto"
	"github.com/xmx/metrics"
)

func NewMetrics(cli *rpclient.Client) cronv3.Tasker {
	return &metricsTask{
		cli: cli,
	}
}

type metricsTask struct {
	cli *rpclient.Client
}

func (mt *metricsTask) Info() cronv3.TaskInfo {
	return cronv3.TaskInfo{
		Name:      "上报系统指标",
		Timeout:   5 * time.Second,
		CronSched: cron.Every(5 * time.Second),
	}
}

func (mt *metricsTask) Call(ctx context.Context) error {
	pushURL := muxproto.AgentToBrokerURL("/api/victoria-metrics/write")
	strURL := pushURL.String()
	cli := mt.cli.HTTPClient()
	opts := &metrics.PushOptions{Client: cli}

	return metrics.PushMetricsExt(ctx, strURL, mt.defaultWrite, opts)
}

func (*metricsTask) defaultWrite(w io.Writer) {
	metrics.WritePrometheus(w, true)
	metrics.WriteFDMetrics(w)
}
