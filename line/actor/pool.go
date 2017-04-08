package actor

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/microfactory/line/line/conf"
	"github.com/pkg/errors"
)

var (
	//ErrPoolExists means a pool exists while it was expected not to
	ErrPoolExists = errors.New("pool already exists")

	//ErrPoolNotExists means a pool was not found while expecting it to exist
	ErrPoolNotExists = errors.New("pool doesn't exist")
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
	}
}

//PoolManager manages pools
type PoolManager struct {
	sqs  SQS
	db   DB
	conf PoolManagerConf
}

//NewPoolManager will setup a pool manager
func NewPoolManager(conf PoolManagerConf, sqs SQS, db DB) *PoolManager {
	return &PoolManager{
		sqs:  sqs,
		db:   db,
		conf: conf,
	}
}

//PoolPK is the primary key of a pool
type PoolPK struct {
	PoolID string `dynamodbav:"pool"`
}

//Pool is an actor that is responsible for scheduling evaluations onto a subset of workers
type Pool struct {
	PoolPK
	QueueURL string `dynamodbav:"que"`
	TTL      int64  `dynamodbav:"ttl"`
}

//FmtScheduleQueueName allows prediction of the scheduling queue name by poolID
func (pm *PoolManager) FmtScheduleQueueName(pk PoolPK) string {
	return fmt.Sprintf("%s-s%s", pm.conf.Deployment, pk.PoolID)
}

//FmtScheduleQueueURL allows predicting the scheduling queue url based on conf
func (pm *PoolManager) FmtScheduleQueueURL(pk PoolPK) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", pm.conf.AWSRegion, pm.conf.AWSAccountID, pm.FmtScheduleQueueName(pk))
}

//updateTTL under the condition that it exists
func (pm *PoolManager) updateTTL(pk PoolPK, ttl int64) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	ttlattr, err := dynamodbattribute.Marshal(ttl)
	if err != nil {
		return errors.Wrap(err, "failed to marshal new ttl")
	}

	if _, err = pm.db.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(pm.conf.PoolsTableName),
		Key:                 ipk,
		UpdateExpression:    aws.String("SET #ttl = :ttl"),
		ConditionExpression: aws.String("attribute_exists(#pool)"),
		ExpressionAttributeNames: map[string]*string{
			"#ttl":  aws.String("ttl"),
			"#pool": aws.String("pool"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":ttl": ttlattr,
		},
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to update item")
		}

		return ErrPoolNotExists
	}

	return nil
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
	pool = &Pool{}
	pool.PoolID, err = GenEntityID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate id")
	}

	var qout *sqs.CreateQueueOutput
	if qout, err = pm.sqs.CreateQueue(&sqs.CreateQueueInput{
		QueueName: aws.String(pm.FmtScheduleQueueName(pool.PoolPK)),
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

	return pool, nil
}

//DisbandPool will mark the pool for ttl garbage collection
func (pm *PoolManager) DisbandPool(pk PoolPK) (err error) {
	if _, err = pm.sqs.DeleteQueue(&sqs.DeleteQueueInput{
		QueueUrl: aws.String(pm.FmtScheduleQueueURL(pk)),
	}); err != nil {
		return errors.Wrap(err, "failed to remove queue")
	}

	ttl := time.Now().Unix() + pm.conf.PoolTTL
	err = pm.updateTTL(pk, ttl)
	if err != nil {
		return errors.Wrap(err, "failed to update ttl")
	}

	return nil
}
