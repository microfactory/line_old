package line

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/pkg/errors"
)

//AllocPK describes the alloc's primary key in the base table
type AllocPK struct {
	PoolID  string `dynamodbav:"pool"`
	AllocID string `dynamodbav:"alloc"`
}

//Alloc represents a planned execution
type Alloc struct {
	AllocPK
	Size     int    `dynamodbav:"size"`
	TTL      int64  `dynamodbav:"ttl"`
	WorkerID string `dynamodbav:"wrk"`
}

var (
	//ErrAllocExists means a alloc exists while it was expected not to
	ErrAllocExists = errors.New("alloc already exists")

	//ErrAllocNotExists means a alloc was not found while expecting it to exist
	ErrAllocNotExists = errors.New("alloc doesn't exist")
)

//PutNewAlloc will put an alloc with the condition the pk doesn't exist yet
func PutNewAlloc(conf *Conf, db DB, alloc *Alloc) (err error) {
	item, err := dynamodbattribute.MarshalMap(alloc)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = db.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(conf.AllocsTableName),
		ConditionExpression: aws.String("attribute_not_exists(tsk)"),
		Item:                item,
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		return ErrAllocExists
	}

	return nil
}
