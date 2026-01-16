package rpclient

import (
	"net/http"
	"strconv"
	"time"
)

type BusinessErrorBody struct {
	Host     string    `json:"host"     xml:"host"`
	Type     string    `json:"type"     xml:"type"`
	Title    string    `json:"title"    xml:"title"`
	Status   int       `json:"status"   xml:"status"`
	Detail   string    `json:"detail"   xml:"detail"`
	Instance string    `json:"instance" xml:"instance"`
	Method   string    `json:"method"   xml:"method"`
	Datetime time.Time `json:"datetime" xml:"datetime"`
}

func (b BusinessErrorBody) String() string {
	return "business error, host='" + b.Host + "'" +
		", method='" + b.Method + "'" +
		", instance='" + b.Instance + "'" +
		", status=" + strconv.FormatInt(int64(b.Status), 10) +
		", detail='" + b.Detail + "'" +
		", datetime='" + b.Datetime.Format(time.RFC3339) + "'"
}

type ResponseError struct {
	Request       *http.Request
	RawBody       []byte
	BusinessError *BusinessErrorBody
}

func (r *ResponseError) Error() string {
	if r.BusinessError != nil {
		return r.BusinessError.String()
	}

	return "response error, host='" + r.Request.Host + "'" +
		", method='" + r.Request.Method + "'" +
		", instance='" + r.Request.RequestURI + "'" +
		", status=" + strconv.FormatInt(int64(r.Request.Response.StatusCode), 10) +
		", detail='" + string(r.RawBody) + "'"
}
