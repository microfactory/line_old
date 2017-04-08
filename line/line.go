package line

import (
	"encoding/json"
	"regexp"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/microfactory/line/line/conf"
)

//DB is our alias for the dynamodb iface
type DB dynamodbiface.DynamoDBAPI

//Handler describes a Lambda handler that matches a specific suffic
type Handler func(conf *conf.Conf, svc *conf.Services, ev json.RawMessage) (interface{}, error)

//Handlers map arn suffixes to actual event handlers
var Handlers = map[*regexp.Regexp]Handler{
	regexp.MustCompile(`-schedule$`): HandleSchedule,
	regexp.MustCompile(`-release$`):  HandleRelease,
	regexp.MustCompile(`-gateway$`):  HandleGateway,
}
