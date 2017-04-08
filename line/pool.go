package line

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/microfactory/line/line/conf"
	"github.com/pkg/errors"
)

//PoolPK describes the pool's primary key in the base table
type PoolPK struct {
	PoolID string `dynamodbav:"pool"`
}

//Pool represents capacity provided by pools
type Pool struct {
	PoolPK
	QueueURL string `dynamodbav:"que"`
	TTL      int64  `dynamodbav:"ttl"`
}

var (
	//ErrPoolExists means a pool exists while it was expected not to
	ErrPoolExists = errors.New("pool already exists")

	//ErrPoolNotExists means a pool was not found while expecting it to exist
	ErrPoolNotExists = errors.New("pool doesn't exist")
)

//PutNewPool will put an pool with the condition the pk doesn't exist yet
func PutNewPool(conf *conf.Conf, db DB, pool *Pool) (err error) {
	item, err := dynamodbattribute.MarshalMap(pool)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = db.PutItem(&dynamodb.PutItemInput{
		TableName:                aws.String(conf.PoolsTableName),
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

//UpdatePoolTTL under the condition that it exists
func UpdatePoolTTL(conf *conf.Conf, db DB, ttl int64, pk PoolPK) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	ttlattr, err := dynamodbattribute.Marshal(ttl)
	if err != nil {
		return errors.Wrap(err, "failed to marshal new ttl")
	}

	if _, err = db.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(conf.PoolsTableName),
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

//GetActivePool will get a pool by its pk but errors if it's disbanded
func GetActivePool(conf *conf.Conf, db DB, pk PoolPK) (pool *Pool, err error) {
	pool, err = GetPool(conf, db, pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pool")
	}

	if pool.TTL > 0 {
		return nil, errors.Wrap(err, "pool has been disbanded")
	}

	return pool, nil
}

//GetPool returns a pool by its primary key
func GetPool(conf *conf.Conf, db DB, pk PoolPK) (pool *Pool, err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal keys map")
	}

	var out *dynamodb.GetItemOutput
	if out, err = db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(conf.PoolsTableName),
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
