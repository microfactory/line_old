package line

import (
	"encoding/json"

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

	logs.Info("received gateway json", zap.String("json", string(ev)))

	return &GatewayResponse{
		StatusCode: 200,
		Body:       "hello, world",
	}, nil
}
