package actor

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/microfactory/line/line/conf"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var (
	//ErrWorkerExists means a pool exists while it was expected not to
	ErrWorkerExists = errors.New("worker already exists")

	//ErrWorkerNotExists means a pool was not found while expecting it to exist
	ErrWorkerNotExists = errors.New("worker doesn't exist")

	//ErrWorkerNotEnoughCapacity is returned when a workers has not enough cap
	ErrWorkerNotEnoughCapacity = errors.New("worker doesn't have enough capacity")
)

//WorkersConf configures the pool manager using the environment
type WorkersConf struct {
	AWSAccountID      string `envconfig:"AWS_ACCOUNT_ID"`
	AWSRegion         string `envconfig:"AWS_REGION"`
	Deployment        string `envconfig:"DEPLOYMENT"`
	WorkersTableName  string `envconfig:"TABLE_NAME_WORKERS"`
	WorkersCapIdxName string `envconfig:"TABLE_IDX_WORKERS_CAP"`
}

//NewWorkersConf will setup a pool manager conf
func NewWorkersConf(cfg *conf.Conf) WorkersConf {
	return WorkersConf{
		AWSAccountID:      cfg.AWSAccountID,
		AWSRegion:         cfg.AWSRegion,
		Deployment:        cfg.Deployment,
		WorkersTableName:  cfg.WorkersTableName,
		WorkersCapIdxName: cfg.WorkersCapIdxName,
	}
}

//Workers manages workers
type Workers struct {
	logs *zap.Logger
	sqs  SQS
	db   DB
	conf WorkersConf
}

//NewWorkers will setup a worker manager
func NewWorkers(conf WorkersConf, sqs SQS, db DB, logs *zap.Logger, pool PoolPK) *Workers {
	return &Workers{
		logs: logs,
		sqs:  sqs,
		db:   db,
		conf: conf,
	}
}

func (ws *Workers) putNewWorker(worker *Worker) (err error) {
	item, err := dynamodbattribute.MarshalMap(worker)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = ws.db.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(ws.conf.WorkersTableName),
		ConditionExpression: aws.String("attribute_not_exists(#wkr)"),
		ExpressionAttributeNames: map[string]*string{
			"#wkr": aws.String("wkr"),
		},
		Item: item,
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		return ErrWorkerExists
	}

	return nil
}

func (ws *Workers) getWorker(pk WorkerPK) (worker *Worker, err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal keys map")
	}

	var out *dynamodb.GetItemOutput
	if out, err = ws.db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(ws.conf.WorkersTableName),
		Key:       ipk,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to get item")
	}

	if out.Item == nil {
		return nil, ErrWorkerNotExists
	}

	worker = &Worker{}
	err = dynamodbattribute.UnmarshalMap(out.Item, worker)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal item")
	}

	return worker, nil
}

func (ws *Workers) delete(pk WorkerPK) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	if _, err = ws.db.DeleteItem(&dynamodb.DeleteItemInput{
		TableName:           aws.String(ws.conf.WorkersTableName),
		Key:                 ipk,
		ConditionExpression: aws.String("attribute_exists(#wrk)"),
		ExpressionAttributeNames: map[string]*string{
			"#wrk": aws.String("wrk"),
		},
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to delete item")
		}

		return ErrWorkerNotExists
	}

	return nil
}

//SubtractCapacity will lower the capacity of a worker but not below zero
func (ws *Workers) SubtractCapacity(pk WorkerPK, cap int64) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	capattr, err := dynamodbattribute.Marshal(cap)
	if err != nil {
		return errors.Wrap(err, "failed to marshal min capacity")
	}

	if _, err = ws.db.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(ws.conf.WorkersTableName),
		Key:                 ipk,
		UpdateExpression:    aws.String(`SET cap = cap - :claim`),
		ConditionExpression: aws.String("cap >= :claim"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":claim": capattr,
		},
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to update item")
		}

		return ErrWorkerNotEnoughCapacity
	}

	return nil
}

//Delete will remove the worker
func (ws *Workers) Delete(pk WorkerPK) (err error) {
	if _, err = ws.sqs.DeleteQueue(&sqs.DeleteQueueInput{
		QueueUrl: aws.String(ws.FmtQueueURL(pk)),
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != sqs.ErrCodeQueueDoesNotExist {
			return errors.Wrap(err, "failed to remove queue")
		}

		return ErrWorkerNotExists
	}

	err = ws.delete(pk)
	if err != nil {
		return errors.Wrap(err, "failed to delete worker")
	}

	return nil
}

//CreateScheduler returns a worker scheduler that allows scheduling
func (ws *Workers) CreateScheduler(pool PoolPK) (s *WorkerScheduler) {
	return &WorkerScheduler{
		workers: ws,
		pool:    pool,
	}
}

//FmtQueueName allows prediction of the scheduling queue name by poolID
func (ws *Workers) FmtQueueName(pk WorkerPK) string {
	return fmt.Sprintf("%s-%s-%s", ws.conf.Deployment, pk.PoolID, pk.WorkerID)
}

//FmtQueueURL allows predicting the scheduling queue url based on conf
func (ws *Workers) FmtQueueURL(pk WorkerPK) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", ws.conf.AWSRegion, ws.conf.AWSAccountID, ws.FmtQueueName(pk))
}

//CreateWorker will insert a new worker
func (ws *Workers) CreateWorker(cap int64, pool PoolPK) (worker *Worker, err error) {
	worker = &Worker{}
	worker.Capacity = cap
	worker.PoolID = pool.PoolID
	worker.WorkerID, err = GenEntityID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate id")
	}

	var qout *sqs.CreateQueueOutput
	if qout, err = ws.sqs.CreateQueue(&sqs.CreateQueueInput{
		QueueName: aws.String(ws.FmtQueueName(worker.WorkerPK)),
	}); err != nil {
		return nil, errors.Wrap(err, "failed to create schedule queue")
	}

	worker.QueueURL = aws.StringValue(qout.QueueUrl)
	worker.TTL = 1
	err = ws.putNewWorker(worker)
	if err != nil {
		return nil, errors.Wrap(err, "failed to put worker")
	}

	return worker, nil
}

//ListWithCapacity returns workers with equal or more of the given capacity
func (ws *Workers) ListWithCapacity(pool PoolPK, cap int64) (workers []*Worker, err error) {

	poolattr, err := dynamodbattribute.Marshal(pool.PoolID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal pool id")
	}

	capattr, err := dynamodbattribute.Marshal(cap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal min capacity")
	}

	var capq *dynamodb.QueryOutput
	if capq, err = ws.db.Query(&dynamodb.QueryInput{
		TableName: aws.String(ws.conf.WorkersTableName),
		IndexName: aws.String(ws.conf.WorkersCapIdxName),
		Limit:     aws.Int64(10),
		KeyConditionExpression: aws.String("#pool = :poolID AND #cap >= :evalSize"),
		ExpressionAttributeNames: map[string]*string{
			"#pool": aws.String("pool"),
			"#cap":  aws.String("cap"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":poolID":   poolattr,
			":evalSize": capattr,
		},
	}); err != nil {
		return nil, errors.Wrap(err, "failed to query workers")
	}

	for _, item := range capq.Items {
		worker := &Worker{}
		err := dynamodbattribute.UnmarshalMap(item, worker)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal worker")
		}

		workers = append(workers, worker)
	}

	return workers, nil
}

//FetchWorker will get an existing worker if it's not disbanded
func (ws *Workers) FetchWorker(pk WorkerPK) (worker *Worker, err error) {
	worker, err = ws.getWorker(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get worker")
	}

	return worker, nil
}
