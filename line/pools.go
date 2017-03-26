package line

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/pkg/errors"
)

//PoolPK describes the pools's primary key in the base table
type PoolPK struct {
	PoolID string `dynamodbav:"pool"` //simple partition key
}

//Pool represents a planned execution
type Pool struct {
	PoolPK
	QueueURL string `dynamodbav:"que"`
	CPUCores int    `dynamodbav:"cpu"`
	MemoryMB int    `dynamodbav:"mem"`
}

var (
	//ErrPoolExists means a pool exists while it was expected not to
	ErrPoolExists = errors.New("pool already exists")

	//ErrPoolNotExists means a pool was not found while expecting it to exist
	ErrPoolNotExists = errors.New("pool doesn't exist")
)

//PutNewPool will put a pool with the condition the pk doesn't exist yet
func PutNewPool(conf *Conf, db DB, pool *Pool) (err error) {
	item, err := dynamodbattribute.MarshalMap(pool)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = db.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(conf.PoolsTableName),
		ConditionExpression: aws.String("attribute_not_exists(#pool)"),
		Item:                item,
		ExpressionAttributeNames: map[string]*string{
			"#pool": aws.String("pool"),
		},
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		return ErrPoolExists
	}

	return nil
}

//DeletePool deletes a worker by pk
func DeletePool(conf *Conf, db DB, ppk PoolPK) (err error) {
	pk, err := dynamodbattribute.MarshalMap(ppk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	if _, err = db.DeleteItem(&dynamodb.DeleteItemInput{
		TableName:           aws.String(conf.PoolsTableName),
		Key:                 pk,
		ConditionExpression: aws.String("attribute_exists(#pool)"),
		ExpressionAttributeNames: map[string]*string{
			"#pool": aws.String("pool"),
		},
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to delete item")
		}

		return ErrPoolNotExists
	}

	return nil
}

//GetPool returns a worker by its primary key
func GetPool(conf *Conf, db DB, ppk PoolPK) (pool *Pool, err error) {
	pk, err := dynamodbattribute.MarshalMap(ppk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal keys map")
	}

	var out *dynamodb.GetItemOutput
	if out, err = db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(conf.PoolsTableName),
		Key:       pk,
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
