package rpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/xmx/aegis-common/muxlink/muxproto"
)

type Client struct {
	cli *http.Client
	log *slog.Logger
}

func NewClient(dialer muxproto.Dialer, log *slog.Logger) *Client {
	tran := newHTTPTransport(dialer, log)
	cli := &http.Client{Transport: tran}

	return &Client{
		cli: cli,
		log: log,
	}
}

func (c *Client) HTTPClient() *http.Client {
	return c.cli
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.cli.Do(req)
}

func (c *Client) Transport() http.RoundTripper {
	return c.cli.Transport
}

func (c *Client) Ping(ctx context.Context) error {
	return c.sendJSON(ctx, http.MethodGet, "/api/health/ping", nil, nil)
}

// PostNetworks 上报网卡信息。
func (c *Client) PostNetworks(ctx context.Context, cards NetworkCards) error {
	data := &requestData{Data: cards}

	return c.sendJSON(ctx, http.MethodPost, "/api/system/network", data, nil)
}

// sendJSON 发出 json 收到 json。
// 如果 data 为 nil 代表没有请求报文。
// 如果 result 为 nil 代表不关心响应的内容（不需解析响应报文）。
//
//goland:noinspection GoUnhandledErrorResult
func (c *Client) sendJSON(ctx context.Context, method, path string, data, result any) error {
	var br io.Reader
	if data != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(data); err != nil {
			return err
		}
		br = buf
	}

	reqURL := muxproto.AgentToBrokerURL(path)
	strURL := reqURL.String()
	req, err := http.NewRequestWithContext(ctx, method, strURL, br)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if br != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.send(req)
	if err != nil {
		return err
	}

	body := res.Body
	defer body.Close()

	// 如果 result 传 nil 则认为不关心响应的内容。
	if body == http.NoBody || result == nil {
		return nil
	}

	return json.NewDecoder(body).Decode(result)
}

//goland:noinspection GoUnhandledErrorResult
func (c *Client) send(req *http.Request) (*http.Response, error) {
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	statusCode := res.StatusCode
	if c.is2xx(statusCode) || c.is3xx(statusCode) {
		return res, nil
	}

	body := res.Body
	defer body.Close()

	respErr := &ResponseError{Request: req}
	raw, err := io.ReadAll(io.LimitReader(body, 4096)) // 最多取 4K 响应报文，避免出现大的响应报文。
	if err != nil {
		return nil, respErr
	}
	respErr.RawBody = raw

	if c.isApplicationJSON(res.Header.Get("Content-Type")) {
		berr := new(BusinessErrorBody)
		if err = json.Unmarshal(raw, berr); err == nil {
			respErr.BusinessError = berr
		}
	}

	return nil, respErr
}

func (*Client) is2xx(code int) bool { return code/100 == 2 }
func (*Client) is3xx(code int) bool { return code/100 == 3 }

func (*Client) isApplicationJSON(contentType string) bool {
	before, _, _ := strings.Cut(contentType, ";")
	before = strings.ToLower(strings.TrimSpace(before))

	return before == "application/json"
}
