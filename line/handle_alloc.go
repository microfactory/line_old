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
	"github.com/microfactory/line/line/client"
	"github.com/pkg/errors"
)

//FindReplicas returns locality information for an evaluation
func FindReplicas(conf *Conf, svc *Services, eval *Eval, pool *Pool) ([]*Replica, error) {
	evalattr, err := dynamodbattribute.MarshalMap(eval)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal eval")
	}

	poolattr, err := dynamodbattribute.MarshalMap(pool)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal eval")
	}

	// Step 1: LOCALITY - Find all workers that have replica and store the zones these replicas are in. If no replicas are found, scheduling will fail
	replicas := []*Replica{}
	if eval.Dataset != "" {
		locqin := &dynamodb.QueryInput{
			TableName: aws.String(conf.ReplicasTableName),
			Limit:     aws.Int64(10),
			KeyConditionExpression: aws.String("#pool = :poolID AND begins_with (#rpl, :datasetID)"),
			ExpressionAttributeNames: map[string]*string{
				"#pool": aws.String("pool"),
				"#rpl":  aws.String("rpl"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":poolID":    poolattr["pool"],
				":datasetID": evalattr["set"],
			},
		}

		var locq *dynamodb.QueryOutput
		if locq, err = svc.DB.Query(locqin); err != nil {
			return nil, errors.Wrap(err, "failed to query replicas")
		}

		for _, item := range locq.Items {
			replica := &Replica{}
			err := dynamodbattribute.UnmarshalMap(item, replica)
			if err != nil {
				svc.Logs.Error("failed to unmarshal replica item", zap.Error(err))
				continue
			}

			replicas = append(replicas, replica)
		}
	}

	return replicas, nil
}

//Schedule will try to query the workers table for available room and conditionally update their capacity if it fits.
func Schedule(conf *Conf, svc *Services, eval *Eval, pool *Pool, replicas []*Replica) (alloc *Alloc, err error) {
	evalattr, err := dynamodbattribute.MarshalMap(eval)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal eval")
	}

	poolattr, err := dynamodbattribute.MarshalMap(pool)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal eval")
	}

	svc.Logs.Info("querying workers for", zap.String("t", fmt.Sprintf("%+v", evalattr)))

	// Step 2: CAPACITY - find workers with enough capacity in a given pool.

	//query workers with enough capacity at this point-in-time
	var capq *dynamodb.QueryOutput
	if capq, err = svc.DB.Query(&dynamodb.QueryInput{
		TableName: aws.String(conf.WorkersTableName),
		IndexName: aws.String(conf.WorkersCapIdxName),
		Limit:     aws.Int64(10),
		KeyConditionExpression: aws.String("#pool = :poolID AND #cap >= :evalSize"),
		ExpressionAttributeNames: map[string]*string{
			"#pool": aws.String("pool"),
			"#cap":  aws.String("cap"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":poolID":   poolattr["pool"],
			":evalSize": evalattr["size"],
		},
	}); err != nil {
		return nil, errors.Wrap(err, "failed to query workers")
	}

	//decode dynamo items into candidate workers
	var candidates []*Worker
	for _, item := range capq.Items {
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

	//@TODO filter candidates with a ttl in the past

	//if there is some locality information available, we would like to choose a worker that is near the data.
	//@TODO can we switch to only using dynamo native ttl expiration? This is more related to worker livelyness? Replica's existence on a worker can be significantly out of date as its ttl is controlled by dynamos native expiration
	if len(replicas) > 0 {
		//@TODO if workers with a replica have capacity, put these on top
		//else put workers in the same zone on top
	}

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

//ReceiveEvals will long poll for scheduling messages on the scheduling queue of the pool
func ReceiveEvals(conf *Conf, svc *Services, pool *Pool) (err error) {
	for {
		var out *sqs.ReceiveMessageOutput
		if out, err = svc.SQS.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(pool.QueueURL),
			VisibilityTimeout:   aws.Int64(1),
			MaxNumberOfMessages: aws.Int64(1),
		}); err != nil {
			svc.Logs.Error("failed to receive message", zap.Error(err))
			return
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

			//if the eval requires specific dataset we can provide locality based scheduling by finding replicas in the pool
			replicas := []*Replica{}
			if eval.Dataset != "" {
				replicas, err = FindReplicas(conf, svc, eval, pool)
				if err != nil {
					svc.Logs.Error("failed to find replicas", zap.Error(err))
					continue
				}
			}

			//find capacity in the pool
			alloc, err := Schedule(conf, svc, eval, pool, replicas)
			if err != nil {
				svc.Logs.Error("eval cannot be scheduled", zap.Error(err))
				continue
			}

			err = PutNewAlloc(conf, svc.DB, alloc)
			if err != nil {
				svc.Logs.Error("failed to put allocation", zap.Error(err))
				continue
			}

			allocPl := &client.Alloc{
				AllocID:  alloc.AllocID,
				PoolID:   pool.PoolID,
				WorkerID: alloc.WorkerID,
				//@TODO fill with information the worker needs:
				// - RAM/CPU limits
				// - Docker image
				// - DatasetID/version
				// - AllocID
			}

			allocPlMsg, err := json.Marshal(allocPl)
			if err != nil {
				svc.Logs.Error("failed to encode alloc message", zap.Error(err))
				continue
			}

			if _, err = svc.SQS.SendMessage(&sqs.SendMessageInput{
				QueueUrl:    aws.String(FmtWorkerQueueURL(conf, pool.PoolID, alloc.WorkerID)),
				MessageBody: aws.String(string(allocPlMsg)),
			}); err != nil {
				svc.Logs.Error("failed to send alloc msg", zap.Error(err))
				continue
			}

			if _, err = svc.SQS.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      aws.String(pool.QueueURL),
				ReceiptHandle: msg.ReceiptHandle,
			}); err != nil {
				svc.Logs.Error("failed to delete eval msg", zap.Error(err))
				continue
			}
		}
	}
}

//HandleAlloc is a Lambda handler that periodically reads from the scheduling queue and queries the workers table for available capacity. If the capacity can be claimed an allocation is created.
func HandleAlloc(conf *Conf, svc *Services, ev json.RawMessage) (res interface{}, err error) {

	doneCh := make(chan struct{})
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

				if pool.TTL > 0 {
					continue //pool is marked for deletion, no evaluations allowed
				}

				svc.Logs.Info("pool", zap.String("pool", fmt.Sprintf("%+v", pool)))
				go ReceiveEvals(conf, svc, pool)
			}
			return true
		}); err != nil {
		return
	}

	//this will block forever while messages are being received for pools concurrently
	<-doneCh

	return ev, nil
}
