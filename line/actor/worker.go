package actor

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"

	"go.uber.org/zap"
)

//WorkerPK uniquely identifies a worker
type WorkerPK struct {
	PoolID   string `dynamodbav:"pool"`
	WorkerID string `dynamodbav:"wrk"`
}

//Worker represents a source of capacity
type Worker struct {
	WorkerPK
	Capacity int64  `dynamodbav:"cap"`
	QueueURL string `dynamodbav:"que"`
	TTL      int64  `dynamodbav:"ttl"`

	conf WorkerManagerConf
	db   DB
	sqs  SQS
	logs *zap.Logger
}

//FmtQueueName allows prediction of the scheduling queue name by poolID
func (w *Worker) FmtQueueName() string {
	return fmt.Sprintf("%s-%s-%s", w.conf.Deployment, w.PoolID, w.WorkerID)
}

//FmtQueueURL allows predicting the scheduling queue url based on conf
func (w *Worker) FmtQueueURL() string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", w.conf.AWSRegion, w.conf.AWSAccountID, w.FmtQueueName())
}

func (w *Worker) delete() (err error) {
	ipk, err := dynamodbattribute.MarshalMap(w.WorkerPK)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	if _, err = w.db.DeleteItem(&dynamodb.DeleteItemInput{
		TableName:           aws.String(w.conf.WorkersTableName),
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
func (w *Worker) SubtractCapacity(cap int64) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(w.WorkerPK)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	capattr, err := dynamodbattribute.Marshal(cap)
	if err != nil {
		return errors.Wrap(err, "failed to marshal min capacity")
	}

	if _, err = w.db.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(w.conf.WorkersTableName),
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
func (w *Worker) Delete() (err error) {
	if _, err = w.sqs.DeleteQueue(&sqs.DeleteQueueInput{
		QueueUrl: aws.String(w.FmtQueueURL()),
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != sqs.ErrCodeQueueDoesNotExist {
			return errors.Wrap(err, "failed to remove queue")
		}

		return ErrWorkerNotExists
	}

	err = w.delete()
	if err != nil {
		return errors.Wrap(err, "failed to delete worker")
	}

	return nil
}
