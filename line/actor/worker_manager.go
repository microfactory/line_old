package actor

import (
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

//WorkerManagerConf configures the pool manager using the environment
type WorkerManagerConf struct {
	AWSAccountID      string `envconfig:"AWS_ACCOUNT_ID"`
	AWSRegion         string `envconfig:"AWS_REGION"`
	Deployment        string `envconfig:"DEPLOYMENT"`
	WorkersTableName  string `envconfig:"TABLE_NAME_WORKERS"`
	WorkersCapIdxName string `envconfig:"TABLE_IDX_WORKERS_CAP"`
}

//NewWorkerManagerConf will setup a pool manager conf
func NewWorkerManagerConf(cfg *conf.Conf) WorkerManagerConf {
	return WorkerManagerConf{
		AWSAccountID:      cfg.AWSAccountID,
		AWSRegion:         cfg.AWSRegion,
		Deployment:        cfg.Deployment,
		WorkersTableName:  cfg.WorkersTableName,
		WorkersCapIdxName: cfg.WorkersCapIdxName,
	}
}

//WorkerManager manages workers
type WorkerManager struct {
	logs *zap.Logger
	sqs  SQS
	db   DB
	conf WorkerManagerConf
}

//NewWorkerManager will setup a worker manager
func NewWorkerManager(conf WorkerManagerConf, sqs SQS, db DB, logs *zap.Logger, pool PoolPK) *WorkerManager {
	return &WorkerManager{
		logs: logs,
		sqs:  sqs,
		db:   db,
		conf: conf,
	}
}

func (wm *WorkerManager) putNewWorker(worker *Worker) (err error) {
	item, err := dynamodbattribute.MarshalMap(worker)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = wm.db.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(wm.conf.WorkersTableName),
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

func (wm *WorkerManager) getWorker(pk WorkerPK) (worker *Worker, err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal keys map")
	}

	var out *dynamodb.GetItemOutput
	if out, err = wm.db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(wm.conf.WorkersTableName),
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

//CreateScheduler returns a worker scheduler that allows scheduling
func (wm *WorkerManager) CreateScheduler(pool PoolPK) (s *WorkerScheduler) {
	return &WorkerScheduler{
		pool: pool,
	}
}

//CreateWorker will insert a new worker
func (wm *WorkerManager) CreateWorker(cap int64, pool PoolPK) (worker *Worker, err error) {
	worker = &Worker{
		conf: wm.conf,
		db:   wm.db,
		sqs:  wm.sqs,
		logs: wm.logs,
	}

	worker.Capacity = cap
	worker.PoolID = pool.PoolID
	worker.WorkerID, err = GenEntityID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate id")
	}

	var qout *sqs.CreateQueueOutput
	if qout, err = wm.sqs.CreateQueue(&sqs.CreateQueueInput{
		QueueName: aws.String(worker.FmtQueueName()),
	}); err != nil {
		return nil, errors.Wrap(err, "failed to create schedule queue")
	}

	worker.QueueURL = aws.StringValue(qout.QueueUrl)
	worker.TTL = 1
	err = wm.putNewWorker(worker)
	if err != nil {
		return nil, errors.Wrap(err, "failed to put worker")
	}

	return worker, nil
}

//ListWithCapacity returns workers with equal or more of the given capacity
func (wm *WorkerManager) ListWithCapacity(pool PoolPK, cap int64) (workers []*Worker, err error) {

	poolattr, err := dynamodbattribute.Marshal(pool.PoolID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal pool id")
	}

	capattr, err := dynamodbattribute.Marshal(cap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal min capacity")
	}

	var capq *dynamodb.QueryOutput
	if capq, err = wm.db.Query(&dynamodb.QueryInput{
		TableName: aws.String(wm.conf.WorkersTableName),
		IndexName: aws.String(wm.conf.WorkersCapIdxName),
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
func (wm *WorkerManager) FetchWorker(pk WorkerPK) (worker *Worker, err error) {
	worker, err = wm.getWorker(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get worker")
	}

	worker.conf = wm.conf
	worker.db = wm.db
	worker.sqs = wm.sqs
	worker.logs = wm.logs
	return worker, nil
}
