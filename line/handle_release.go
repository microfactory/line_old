package line

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func releaseReplicas(conf *Conf, svc *Services, pool *Pool) (err error) {
	condAttr, err := dynamodbattribute.MarshalMap(Replica{
		TTL:       time.Now().Unix(),
		ReplicaPK: ReplicaPK{PoolID: pool.PoolID},
	})
	if err != nil {
		return errors.Wrap(err, "failed to marshal replica condition")
	}

	var out *dynamodb.QueryOutput
	if out, err = svc.DB.Query(&dynamodb.QueryInput{
		TableName:              aws.String(conf.ReplicasTableName),
		IndexName:              aws.String(conf.ReplicasTTLIdxName),
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
		return errors.Wrap(err, "failed to query replicas")
	}

	svc.Logs.Info("replicas expired", zap.Int("n", len(out.Items)))
	for _, item := range out.Items {
		replica := &Replica{}
		err = dynamodbattribute.UnmarshalMap(item, replica)
		if err != nil {
			svc.Logs.Error("failed to unmarshal expired replica", zap.Error(err))
			continue
		}

		err = DeleteReplica(conf, svc.DB, replica.ReplicaPK)
		if err != nil {
			svc.Logs.Error("failed to delete replica", zap.String("replica", fmt.Sprintf("%+v", replica.ReplicaPK)), zap.Error(err))
		}
	}

	return nil
}

func releaseWorker(conf *Conf, svc *Services, pool *Pool) (err error) {
	condAttr, err := dynamodbattribute.MarshalMap(Worker{
		TTL:      time.Now().Unix(),
		WorkerPK: WorkerPK{PoolID: pool.PoolID},
	})
	if err != nil {
		return errors.Wrap(err, "failed to marshal worker condition")
	}

	var out *dynamodb.QueryOutput
	if out, err = svc.DB.Query(&dynamodb.QueryInput{
		TableName:              aws.String(conf.WorkersTableName),
		IndexName:              aws.String(conf.WorkersTTLIdxName),
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
		return errors.Wrap(err, "failed to query workers")
	}

	svc.Logs.Info("workers expired", zap.Int("n", len(out.Items)))
	for _, item := range out.Items {
		worker := &Worker{}
		err = dynamodbattribute.UnmarshalMap(item, worker)
		if err != nil {
			svc.Logs.Error("failed to unmarshal workers replica", zap.Error(err))
			continue
		}

		if _, err = svc.SQS.DeleteQueue(&sqs.DeleteQueueInput{
			QueueUrl: aws.String(FmtWorkerQueueURL(conf, worker.PoolID, worker.WorkerID)),
		}); err != nil {
			svc.Logs.Error("failed to remove worker queue", zap.Error(err))
			continue
		}

		err = DeleteWorker(conf, svc.DB, worker.WorkerPK)
		if err != nil {
			svc.Logs.Error("failed to delete worker", zap.String("worker", fmt.Sprintf("%+v", worker.WorkerPK)), zap.Error(err))
		}
	}

	return nil
}

func releaseAllocs(conf *Conf, svc *Services, pool *Pool) (err error) {
	condAttr, err := dynamodbattribute.MarshalMap(Alloc{
		AllocPK: AllocPK{PoolID: pool.PoolID},
		TTL:     time.Now().Unix(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to marshal alloc condition")
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
		return errors.Wrap(err, "failed to query allocations")
	}

	svc.Logs.Info("allocations expired", zap.Int("n", len(out.Items)))
	for _, item := range out.Items {
		alloc := &Alloc{}
		err = dynamodbattribute.UnmarshalMap(item, alloc)
		if err != nil {
			svc.Logs.Error("failed to unmarshal expired alloc", zap.Error(err))
			continue
		}

		svc.Logs.Info("releasing alloc", zap.String("alloc", fmt.Sprintf("%+v", alloc)), zap.String("item", fmt.Sprintf("%+v", item)))

		if alloc.WorkerID == "" {
			svc.Logs.Error("allocation has no worker field")
			continue
		}

		if alloc.Eval == nil {
			svc.Logs.Error("allocation has no eval")
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

		allocSize, err := dynamodbattribute.Marshal(alloc.Eval.Size)
		if err != nil {
			svc.Logs.Error("failed to marshal alloc size", zap.Error(err))
			continue
		}

		evalMsg, err := json.Marshal(alloc.Eval)
		if err != nil {
			svc.Logs.Error("failed to marshal eval msg", zap.Error(err))
			continue
		}

		if alloc.Eval.Retry >= conf.MaxRetry {
			if _, err = svc.SQS.SendMessage(&sqs.SendMessageInput{
				QueueUrl:    aws.String(conf.ScheduleDLQueueURL),
				MessageBody: aws.String(string(evalMsg)),
			}); err != nil {
				svc.Logs.Error("failed to retry eval on dead letter queue", zap.Error(err))
				continue
			}
		} else {
			if _, err = svc.SQS.SendMessage(&sqs.SendMessageInput{
				QueueUrl:    aws.String(pool.QueueURL),
				MessageBody: aws.String(string(evalMsg)),
			}); err != nil {

				aerr, ok := err.(awserr.Error)
				if !ok || aerr.Code() != sqs.ErrCodeQueueDoesNotExist {
					svc.Logs.Error("failed to re-send eval on pool queue", zap.Error(err))
					continue
				}

				//else we assume the scheduling queue was deleted because the pool itself is disbaned, we dont try to reschedule
			}
		}

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
				":allocSize": allocSize,
			},
		}); err != nil {
			//@TODO worker may have been removed, due do worker ttl or otherwise
			svc.Logs.Error("failed to release capacity back to worker", zap.Error(err))
			continue
		}
	}

	return nil
}

//HandleRelease is a Lambda handler that periodically queries a pool's expired allocations, replicas and workers
func HandleRelease(conf *Conf, svc *Services, ev json.RawMessage) (res interface{}, err error) {

	if err = svc.DB.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(conf.PoolsTableName),
	},
		func(page *dynamodb.ScanOutput, lastPage bool) bool {
			for _, item := range page.Items {
				pool := &Pool{}
				err := dynamodbattribute.UnmarshalMap(item, pool)
				if err != nil {
					svc.Logs.Error("failed to unmarshal replica item", zap.Error(err))
					continue
				}

				//@TODO do this concurrently(?)
				err = releaseAllocs(conf, svc, pool)
				if err != nil {
					svc.Logs.Error("failed to release pool allocs", zap.String("pool", pool.PoolID), zap.Error(err))
					continue
				}

				//@TODO do this concurrently(?)
				err = releaseReplicas(conf, svc, pool)
				if err != nil {
					svc.Logs.Error("failed to release pool replicas", zap.String("pool", pool.PoolID), zap.Error(err))
					continue
				}

				//@TODO do this concurrently(?)
				err = releaseWorker(conf, svc, pool)
				if err != nil {
					svc.Logs.Error("failed to release workers", zap.String("pool", pool.PoolID), zap.Error(err))
					continue
				}
			}

			return true
		}); err != nil {
		return
	}

	return ev, nil
}
