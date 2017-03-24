package line

import (
	"encoding/json"
	"regexp"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"go.uber.org/zap"
)

//DB is our alias for the dynamodb iface
type DB dynamodbiface.DynamoDBAPI

//Services hold our backend services
type Services struct {
	SQS  sqsiface.SQSAPI           //message queues
	SFN  sfniface.SFNAPI           //step function state machines
	DB   dynamodbiface.DynamoDBAPI //dynamodb nosql database
	Logs *zap.Logger               //logging service
}

//Conf holds our configuration taken from the environment
type Conf struct {
	Deployment         string `envconfig:"DEPLOYMENT"`
	StateMachineARN    string `envconfig:"STATE_MACHINE_ARN"`
	RunActivityARN     string `envconfig:"RUN_ACTIVITY_ARN"`
	AWSAccountID       string `envconfig:"AWS_ACCOUNT_ID"`
	AWSAccessKeyID     string `envconfig:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string `envconfig:"AWS_SECRET_ACCESS_KEY"`
	AWSRegion          string `envconfig:"AWS_REGION"`
	PoolQueueURL       string `envconfig:"POOL_QUEUE_URL"`
	StripBaseMappings  int    `envconfig:"STRIP_BASE_MAPPINGS"`
	PoolsTableName     string `envconfig:"TABLE_NAME_POOLS"`
}

//Handler describes a Lambda handler that matches a specific suffic
type Handler func(conf *Conf, svc *Services, ev json.RawMessage) (interface{}, error)

//Handlers map arn suffixes to actual event handlers
var Handlers = map[*regexp.Regexp]Handler{
	regexp.MustCompile(`-dispatch$`): HandleDispatch,
	regexp.MustCompile(`-gateway$`):  HandleGateway,
}
