package line

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/microfactory/line/line/conf"
	"github.com/pkg/errors"
)

//WorkerPK uniquely identifies a worker
type WorkerPK struct {
	PoolID   string `dynamodbav:"pool"`
	WorkerID string `dynamodbav:"wrk"`
}

//Worker represents a source of capacity
type Worker struct {
	WorkerPK
	Capacity int    `dynamodbav:"cap"`
	QueueURL string `dynamodbav:"que"`
	TTL      int64  `dynamodbav:"ttl"`
}

var (
	//ErrWorkerExists means a worker exists while it was expected not to
	ErrWorkerExists = errors.New("worker already exists")

	//ErrWorkerNotExists means a worker was not found while expecting it to exist
	ErrWorkerNotExists = errors.New("worker doesn't exist")
)

//PutNewWorker will put an worker with the condition the pk doesn't exist yet
func PutNewWorker(conf *conf.Conf, db DB, worker *Worker) (err error) {
	item, err := dynamodbattribute.MarshalMap(worker)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = db.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(conf.WorkersTableName),
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

//DeleteWorker deletes a worker by pk
func DeleteWorker(conf *conf.Conf, db DB, pk WorkerPK) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	if _, err = db.DeleteItem(&dynamodb.DeleteItemInput{
		TableName:           aws.String(conf.WorkersTableName),
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

//UpdateWorkerTTL under the condition that it exists
func UpdateWorkerTTL(conf *conf.Conf, db DB, ttl int64, pk WorkerPK) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	ttlattr, err := dynamodbattribute.Marshal(ttl)
	if err != nil {
		return errors.Wrap(err, "failed to marshal new ttl")
	}

	if _, err = db.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(conf.WorkersTableName),
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

		return ErrWorkerNotExists
	}

	return nil
}

//GetWorker returns a worker by its primary key
func GetWorker(conf *conf.Conf, db DB, pk WorkerPK) (worker *Worker, err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal keys map")
	}

	var out *dynamodb.GetItemOutput
	if out, err = db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(conf.WorkersTableName),
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
