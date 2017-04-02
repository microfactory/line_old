package line

import (
	"encoding/json"
	"regexp"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"go.uber.org/zap"
)

//DB is our alias for the dynamodb iface
type DB dynamodbiface.DynamoDBAPI

//Services hold our backend services
type Services struct {
	SQS  sqsiface.SQSAPI           //message queues
	DB   dynamodbiface.DynamoDBAPI //dynamodb nosql database
	Logs *zap.Logger               //logging service
}

//Conf holds our configuration taken from the environment
type Conf struct {
	Deployment         string `envconfig:"DEPLOYMENT"`
	AWSAccountID       string `envconfig:"AWS_ACCOUNT_ID"`
	AWSAccessKeyID     string `envconfig:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string `envconfig:"AWS_SECRET_ACCESS_KEY"`
	AWSRegion          string `envconfig:"AWS_REGION"`
	StripBaseMappings  int    `envconfig:"STRIP_BASE_MAPPINGS"`

	AllocTTL int64 `envconfig:"ALLOC_TTL"`
	MaxRetry int   `envconfig:"MAX_RETRY"`

	ScheduleQueueURL   string `envconfig:"SCHEDULE_QUEUE_URL"`
	ScheduleDLQueueURL string `envconfig:"SCHEDULE_DLQUEUE_URL"`

	PoolsTableName    string `envconfig:"TABLE_NAME_POOLS"`
	ReplicasTableName string `envconfig:"TABLE_NAME_REPLICAS"`
	WorkersTableName  string `envconfig:"TABLE_NAME_WORKERS"`
	WorkersCapIdxName string `envconfig:"TABLE_IDX_WORKERS_CAP"`
	AllocsTableName   string `envconfig:"TABLE_NAME_ALLOCS"`
	AllocsTTLIdxName  string `envconfig:"TABLE_IDX_ALLOCS_TTL"`
}

//Handler describes a Lambda handler that matches a specific suffic
type Handler func(conf *Conf, svc *Services, ev json.RawMessage) (interface{}, error)

//Handlers map arn suffixes to actual event handlers
var Handlers = map[*regexp.Regexp]Handler{
	regexp.MustCompile(`-alloc$`):   HandleAlloc,
	regexp.MustCompile(`-release$`): HandleRelease,
	regexp.MustCompile(`-gateway$`): HandleGateway,
}
