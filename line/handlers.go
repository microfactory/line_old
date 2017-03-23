package line

import (
	"encoding/json"
	"regexp"

	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
)

//Handler describes a Lambda handler that matches a specific suffic
type Handler func(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (interface{}, error)

//Handlers map arn suffixes to actual event handlers
var Handlers = map[*regexp.Regexp]Handler{
	regexp.MustCompile(`-dispatch$`): HandleDispatch,
	regexp.MustCompile(`-gateway$`):  HandleGateway,
}
