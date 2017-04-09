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
	//ErrPoolExists means a pool exists while it was expected not to
	ErrPoolExists = errors.New("pool already exists")

	//ErrPoolNotExists means a pool was not found while expecting it to exist
	ErrPoolNotExists = errors.New("pool doesn't exist")

	//ErrPoolNotEnoughCapacity is returned when the pool doesn't have enough cap
	ErrPoolNotEnoughCapacity = errors.New("pool doesn't have enough capacity")
)

//PoolManagerConf configures the pool manager using the environment
type PoolManagerConf struct {
	AWSAccountID   string `envconfig:"AWS_ACCOUNT_ID"`
	AWSRegion      string `envconfig:"AWS_REGION"`
	PoolTTL        int64  `envconfig:"POOL_TTL"`
	Deployment     string `envconfig:"DEPLOYMENT"`
	PoolsTableName string `envconfig:"TABLE_NAME_POOLS"`
}

//NewPoolManagerConf will setup a pool manager conf
func NewPoolManagerConf(cfg *conf.Conf) PoolManagerConf {
	return PoolManagerConf{
		PoolTTL:        cfg.PoolTTL,
		Deployment:     cfg.Deployment,
		PoolsTableName: cfg.PoolsTableName,
		AWSAccountID:   cfg.AWSAccountID,
		AWSRegion:      cfg.AWSRegion,
	}
}

//PoolManager manages pools
type PoolManager struct {
	logs *zap.Logger
	sqs  SQS
	db   DB
	conf PoolManagerConf
}

//NewPoolManager will setup a pool manager
func NewPoolManager(conf PoolManagerConf, sqs SQS, db DB, logs *zap.Logger) *PoolManager {
	return &PoolManager{
		logs: logs,
		sqs:  sqs,
		db:   db,
		conf: conf,
	}
}

func (pm *PoolManager) getPool(pk PoolPK) (pool *Pool, err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal keys map")
	}

	var out *dynamodb.GetItemOutput
	if out, err = pm.db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(pm.conf.PoolsTableName),
		Key:       ipk,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to get item")
	}

	if out.Item == nil {
		return nil, ErrPoolNotExists
	}

	pool = &Pool{}
	err = dynamodbattribute.UnmarshalMap(out.Item, pool)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal item")
	}

	return pool, nil
}

func (pm *PoolManager) putNewPool(pool *Pool) (err error) {
	item, err := dynamodbattribute.MarshalMap(pool)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = pm.db.PutItem(&dynamodb.PutItemInput{
		TableName:                aws.String(pm.conf.PoolsTableName),
		ConditionExpression:      aws.String("attribute_not_exists(#pool)"),
		ExpressionAttributeNames: map[string]*string{"#pool": aws.String("pool")},
		Item: item,
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		return ErrPoolExists
	}

	return nil
}

//CreatePool creates a new pool
func (pm *PoolManager) CreatePool() (pool *Pool, err error) {
	pool = &Pool{
		conf: pm.conf,
		db:   pm.db,
		sqs:  pm.sqs,
		logs: pm.logs,
	}

	pool.PoolID, err = GenEntityID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate id")
	}

	var qout *sqs.CreateQueueOutput
	if qout, err = pm.sqs.CreateQueue(&sqs.CreateQueueInput{
		QueueName: aws.String(pool.FmtScheduleQueueName()),
	}); err != nil {
		return nil, errors.Wrap(err, "failed to create schedule queue")
	}

	pool.QueueURL = aws.StringValue(qout.QueueUrl)
	pool.TTL = 1
	err = pm.putNewPool(pool)
	if err != nil {
		return nil, errors.Wrap(err, "failed to put pool")
	}

	return pool, nil
}

//FetchPool will get an existing pool if it's not disbanded
func (pm *PoolManager) FetchPool(pk PoolPK) (pool *Pool, err error) {
	pool, err = pm.getPool(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pool")
	}

	if pool.TTL != 1 {
		return nil, ErrPoolNotExists
	}

	pool.conf = pm.conf
	pool.db = pm.db
	pool.sqs = pm.sqs
	pool.logs = pm.logs
	return pool, nil
}
