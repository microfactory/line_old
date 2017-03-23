package line

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// GatewayRequest represents an Amazon API Gateway Proxy Event.
type GatewayRequest struct {
	HTTPMethod            string
	Headers               map[string]string
	Resource              string
	PathParameters        map[string]string
	Path                  string
	QueryStringParameters map[string]string
	Body                  string
	IsBase64Encoded       bool
	StageVariables        map[string]string
}

//GatewayResponse is returned to the API Gateway
type GatewayResponse struct {
	StatusCode int               `json:"statusCode"`
	Body       string            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

//HandleGateway takes invocations from the API Gateway and handles them as HTTP requests to return HTTP responses based on restful principles
func HandleGateway(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (res interface{}, err error) {
	req := &GatewayRequest{}
	err = json.Unmarshal(ev, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode gateway request")
	}

	//parse path
	loc, err := url.Parse(req.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse request path")
	}

	//strip path base
	if conf.StripBaseMappings > 0 {
		comps := strings.SplitN(
			strings.TrimLeft(loc.Path, "/"),
			"/", conf.StripBaseMappings+1)
		if len(comps) >= conf.StripBaseMappings {
			loc.Path = "/" + strings.Join(comps[conf.StripBaseMappings:], "/")
		} else {
			loc.Path = "/"
		}
	}

	q := loc.Query()
	for k, param := range req.QueryStringParameters {
		q.Set(k, param)
	}

	loc.RawQuery = q.Encode()
	r, err := http.NewRequest(req.HTTPMethod, loc.String(), bytes.NewBufferString(req.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to turn event %+v into http request: %v", req, err)
	}

	for k, val := range req.Headers {
		for _, v := range strings.Split(val, ",") {
			r.Header.Add(k, strings.TrimSpace(v))
		}
	}

	w := &bufferedResponse{
		statusCode: http.StatusOK, //like standard lib, assume 200
		header:     http.Header{},
		Buffer:     bytes.NewBuffer(nil),
	}

	Mux(conf, logs, sess).ServeHTTP(w, r)

	resp := &GatewayResponse{
		StatusCode: w.statusCode,
		Body:       w.Buffer.String(),
		Headers:    map[string]string{},
	}

	for k, v := range w.header {
		resp.Headers[k] = strings.Join(v, ",")
	}

	return resp, nil
}

//bufferedResponse implements the response writer interface but buffers the body which is necessary for the creating a JSON formatted Lambda response anyway
type bufferedResponse struct {
	statusCode int
	header     http.Header
	*bytes.Buffer
}

func (br *bufferedResponse) Header() http.Header    { return br.header }
func (br *bufferedResponse) WriteHeader(status int) { br.statusCode = status }
