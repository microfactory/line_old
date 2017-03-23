package line

import (
	"encoding/json"
	"regexp"

	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
)

//Conf holds our configuration taken from the environment
type Conf struct {
	Deployment         string `envconfig:"DEPLOYMENT"`
	StateMachineARN    string `envconfig:"STATE_MACHINE_ARN"`
	RunActivityARN     string `envconfig:"RUN_ACTIVITY_ARN"`
	AWSAccessKeyID     string `envconfig:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string `envconfig:"AWS_SECRET_ACCESS_KEY"`
	AWSRegion          string `envconfig:"AWS_REGION"`
	PoolQueueURL       string `envconfig:"POOL_QUEUE_URL"`
	StripBaseMappings  int    `envconfig:"STRIP_BASE_MAPPINGS"`
}

//Handler describes a Lambda handler that matches a specific suffic
type Handler func(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (interface{}, error)

//Handlers map arn suffixes to actual event handlers
var Handlers = map[*regexp.Regexp]Handler{
	regexp.MustCompile(`-dispatch$`): HandleDispatch,
	regexp.MustCompile(`-gateway$`):  HandleGateway,
}
