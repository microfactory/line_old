package line

import (
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//HandleRelease is a Lambda handler that periodically queries a pools expired allocations and releases the capacity back to the worker table
func HandleRelease(conf *Conf, svc *Services, ev json.RawMessage) (res interface{}, err error) {

	condAttr, err := dynamodbattribute.MarshalMap(Alloc{
		AllocPK: AllocPK{PoolID: "default"}, //@TODO do for each pool
		TTL:     time.Now().Unix(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal alloc condition")
	}

	var out *dynamodb.QueryOutput
	if out, err = svc.DB.Query(&dynamodb.QueryInput{
		TableName:              aws.String(conf.AllocsTableName),
		IndexName:              aws.String(conf.AllocsTTLIdxName),
		KeyConditionExpression: aws.String("#pool = :poolID AND #ttl < :now"),
		ExpressionAttributeNames: map[string]*string{
			"#pool": aws.String("pool"),
			"#ttl":  aws.String("ttl"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":poolID": condAttr["pool"],
			":now":    condAttr["ttl"],
		},
	}); err != nil {
		return nil, errors.Wrap(err, "failed to query allocations")
	}

	svc.Logs.Info("allocations expired", zap.Int("n", len(out.Items)))

	return ev, nil
}
