package actor

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
)

//PoolPK is the primary key of a pool
type PoolPK struct {
	PoolID string `dynamodbav:"pool"`
}

//Pool is an actor that is responsible for scheduling evaluations onto a subset of workers
type Pool struct {
	PoolPK
	QueueURL string `dynamodbav:"que"`
	TTL      int64  `dynamodbav:"ttl"`

	conf PoolManagerConf
	db   DB
	sqs  SQS
	logs *zap.Logger
}

//FmtScheduleQueueName allows prediction of the scheduling queue name by poolID
func (p *Pool) FmtScheduleQueueName() string {
	return fmt.Sprintf("%s-s%s", p.conf.Deployment, p.PoolID)
}

//FmtScheduleQueueURL allows predicting the scheduling queue url based on conf
func (p *Pool) FmtScheduleQueueURL() string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", p.conf.AWSRegion, p.conf.AWSAccountID, p.FmtScheduleQueueName())
}

func (p *Pool) updateTTL(ttl int64) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(p.PoolPK)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	ttlattr, err := dynamodbattribute.Marshal(ttl)
	if err != nil {
		return errors.Wrap(err, "failed to marshal new ttl")
	}

	if _, err = p.db.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(p.conf.PoolsTableName),
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

//Disband will mark the pool for final deletion through a background process
func (p *Pool) Disband() (err error) {
	if _, err = p.sqs.DeleteQueue(&sqs.DeleteQueueInput{
		QueueUrl: aws.String(p.FmtScheduleQueueURL()),
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != sqs.ErrCodeQueueDoesNotExist {
			return errors.Wrap(err, "failed to remove queue")
		}

		return ErrPoolNotExists
	}

	ttl := time.Now().Unix() + p.conf.PoolTTL
	err = p.updateTTL(ttl)
	if err != nil {
		return errors.Wrap(err, "failed to update ttl")
	}

	return nil
}

//HandleEvals will start receiving eval messages until doneCh is closed
func (p *Pool) HandleEvals(s Scheduler, waitSeconds int64, doneCh <-chan struct{}) {
	for {
		out, err := p.sqs.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(p.QueueURL),
			VisibilityTimeout:   aws.Int64(1),
			MaxNumberOfMessages: aws.Int64(10),
			WaitTimeSeconds:     aws.Int64(waitSeconds),
		})
		if err != nil {
			aerr, ok := err.(awserr.Error)
			if !ok || aerr.Code() != sqs.ErrCodeQueueDoesNotExist {
				p.logs.Error("failed to receive message", zap.Error(err))
			}

			return //queue was removed by another process
		}

		//don't handle any more messages if we've received a done
		select {
		case <-doneCh:
			return
		default:
		}

		//schedule each eval
		for _, msg := range out.Messages {
			eval := &Eval{}
			err = json.Unmarshal([]byte(aws.StringValue(msg.Body)), eval)
			if err != nil {
				p.logs.Error("failed to unmarshal eval msg", zap.Error(err))
				continue
			}

			err = s.Schedule(eval)
			if err != nil {
				p.logs.Error("failed to schedule eval", zap.Error(err))
				continue
			}

			if _, err = p.sqs.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      aws.String(p.QueueURL),
				ReceiptHandle: msg.ReceiptHandle,
			}); err != nil {
				p.logs.Error("failed to delete eval msg", zap.Error(err))
				continue
			}
		}
	}
}

//ScheduleEval will plan an evaluation for scheduling on the pool
func (p *Pool) ScheduleEval(eval *Eval) (err error) {
	msg, err := json.Marshal(eval)
	if err != nil {
		return errors.Wrap(err, "failed to encode eval message")
	}

	if _, err := p.sqs.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(p.QueueURL),
		MessageBody: aws.String(string(msg)),
	}); err != nil {
		return errors.Wrap(err, "failed to send eval message")
	}

	return nil
}
