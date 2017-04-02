package line

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
)

//Schedule will try to query the workers table for available room and conditionally update their capacity if it fits.
func Schedule(conf *Conf, svc *Services, eval *Eval) (alloc *Alloc, err error) {

	// Step 1: LOCALITY - Find all workers that have a replica and store the zones these replicas are in. If no replicas are found, scheduling will fail

	// Step 2: CAPACITY - find workers with enough capacity that are near these replicas. Prefferably on worker that has the replica, if not a worker in a zone nearby.

	//query our secondary index for workers with capacity of at least the size of the eval we require. @TODO apply conditional filters on results for additional selection
	evalattr, err := dynamodbattribute.MarshalMap(eval)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal eval")
	}

	svc.Logs.Info("querying workers for", zap.String("t", fmt.Sprintf("%+v", evalattr)))

	//query workers with enough capacity at this point-in-time
	var qout *dynamodb.QueryOutput
	if qout, err = svc.DB.Query(&dynamodb.QueryInput{
		TableName: aws.String(conf.WorkersTableName),
		IndexName: aws.String(conf.WorkersCapIdxName),
		Limit:     aws.Int64(10),
		KeyConditionExpression: aws.String("#pool = :poolID AND #cap >= :evalSize"),
		ExpressionAttributeNames: map[string]*string{
			"#pool": aws.String("pool"),
			"#cap":  aws.String("cap"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":poolID":   evalattr["pool"],
			":evalSize": evalattr["size"],
		},
	}); err != nil {
		return nil, errors.Wrap(err, "failed to query workers")
	}

	//decode dynamo items into candidate workers
	var candidates []*Worker
	for _, item := range qout.Items {
		cand := &Worker{}
		err := dynamodbattribute.UnmarshalMap(item, cand)
		if err != nil {
			svc.Logs.Error("failed to unmarshal item", zap.Error(err))
			continue
		}

		candidates = append(candidates, cand)
	}

	//sort by workers with the highest capacity (spread) @TODO add a way of placing on lowest capacity, this will create more contention but allows more room for large placements in the future
	svc.Logs.Info("received candidate workers", zap.Int("candidates", len(candidates)))
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Capacity >= candidates[j].Capacity
	})

	//if we have no candidates to begin we return an error en hope it will be better in the future
	if len(candidates) < 1 {
		return nil, errors.Errorf("not enough capacity")
	}

	//then continue updating the selected worker's capacity to claim it, after this the capacity is allocated
	worker := candidates[0]
	pk, err := dynamodbattribute.MarshalMap(worker.WorkerPK)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal worker pk")
	}

	svc.Logs.Info("claim capacity of worker", zap.String("pool", worker.PoolID), zap.String("wrk", worker.WorkerID))
	if _, err = svc.DB.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(conf.WorkersTableName),
		Key:                 pk,
		UpdateExpression:    aws.String(`SET cap = cap - :claim`),
		ConditionExpression: aws.String("cap >= :claim"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":claim": evalattr["size"],
		},
	}); err != nil {
		return nil, errors.Wrap(err, "failed to update worker capacity")
	}

	idb := make([]byte, 10)
	_, err = rand.Read(idb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate random alloc id")
	}

	eval.Retry = eval.Retry + 1
	alloc = &Alloc{
		AllocPK:  AllocPK{PoolID: worker.PoolID, AllocID: hex.EncodeToString(idb)},
		TTL:      time.Now().Unix() + conf.AllocTTL,
		WorkerID: worker.WorkerID,
		Eval:     eval,
	}

	return alloc, nil
}

//DeleteEvalMsg removes a message from the schedule queue
func DeleteEvalMsg(conf *Conf, svc *Services, msg *sqs.Message) (err error) {
	if _, err = svc.SQS.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      aws.String(conf.ScheduleQueueURL),
		ReceiptHandle: msg.ReceiptHandle,
	}); err != nil {
		return err
	}
	return nil
}

//HandleAlloc is a Lambda handler that periodically reads from the scheduling queue and queries the workers table for available capacity. If the capacity can be claimed an allocation is created.
func HandleAlloc(conf *Conf, svc *Services, ev json.RawMessage) (res interface{}, err error) {

	//@TODO do for each pool (concurrently)
	for {
		var out *sqs.ReceiveMessageOutput
		if out, err = svc.SQS.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(conf.ScheduleQueueURL),
			VisibilityTimeout:   aws.Int64(1),
			MaxNumberOfMessages: aws.Int64(1),
		}); err != nil {
			svc.Logs.Error("failed to receive message", zap.Error(err))
			continue
		}

		for _, msg := range out.Messages {
			svc.Logs.Info("received schedule msg", zap.String("msg", msg.String()))

			eval := &Eval{}
			err = json.Unmarshal([]byte(aws.StringValue(msg.Body)), eval)
			if err != nil {
				svc.Logs.Error("failed to unmarshal eval", zap.Error(err))
				continue
			}

			if eval.Size < 1 {
				eval.Size = 1
			}

			if eval.Pool == "" {
				eval.Pool = "default"
			}

			alloc, err := Schedule(conf, svc, eval)
			if err != nil {
				svc.Logs.Error("eval cannot be scheduled", zap.Error(err))
				continue
			}

			err = PutNewAlloc(conf, svc.DB, alloc)
			if err != nil {
				svc.Logs.Error("failed to put allocation", zap.Error(err))
				continue
			}

			err = DeleteEvalMsg(conf, svc, msg)
			if err != nil {
				svc.Logs.Error("failed to delete eval msg", zap.Error(err))
				continue
			}
		}
	}

}
