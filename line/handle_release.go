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

//HandleRelease is a Lambda handler that periodically queries a pool's expired allocations and releases the capacity back to the worker table
func HandleRelease(conf *Conf, svc *Services, ev json.RawMessage) (res interface{}, err error) {

	//@TODO do for each pool, concurrently

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
	for _, item := range out.Items {
		alloc := &Alloc{}
		err = dynamodbattribute.UnmarshalMap(item, alloc)
		if err != nil {
			svc.Logs.Error("failed to unmarshal expired alloc", zap.Error(err))
			continue
		}

		if alloc.WorkerID == "" {
			svc.Logs.Error("allocation has no worker field")
			continue
		}

		wpk, err := dynamodbattribute.MarshalMap(WorkerPK{PoolID: alloc.PoolID, WorkerID: alloc.WorkerID})
		if err != nil {
			svc.Logs.Error("failed to marshal worker pk", zap.Error(err))
			continue
		}

		apk, err := dynamodbattribute.MarshalMap(alloc.AllocPK)
		if err != nil {
			svc.Logs.Error("failed to marshal worker pk", zap.Error(err))
			continue
		}

		//@TODO place back onto schedule queue unless the result field is set
		// fetch task back and place on schedule queue
		// if _, err = svc.SQS.SendMessage(&sqs.SendMessageInput{
		// 	QueueUrl: aws.String(conf.ScheduleQueueURL),
		// }); err != nil {
		// 	svc.Logs.Error("failed to re-send task to scheduling queue", zap.Error(err))
		// 	continue
		// }

		if _, err = svc.DB.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(conf.AllocsTableName),
			Key:       apk,
		}); err != nil {
			svc.Logs.Error("failed to delete allocation, may never release", zap.Error(err), zap.String("double_release", alloc.WorkerID))
			continue
		}

		if _, err := svc.DB.UpdateItem(&dynamodb.UpdateItemInput{
			TableName:        aws.String(conf.WorkersTableName),
			Key:              wpk,
			UpdateExpression: aws.String(`SET cap = cap + :allocSize`),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":allocSize": item["size"],
			},
		}); err != nil {
			//@TODO worker may have been removed, due do worker ttl or otherwise
			svc.Logs.Error("failed to release capacity back to worker", zap.Error(err))
			continue
		}
	}

	return ev, nil
}
