package line

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/microfactory/line/line/conf"
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
	TTL      int64  `dynamodbav:"ttl"`
	WorkerID string `dynamodbav:"wrk"`
	Eval     *Eval  `dynamodbav:"eval"`
}

var (
	//ErrAllocExists means a alloc exists while it was expected not to
	ErrAllocExists = errors.New("alloc already exists")

	//ErrAllocNotExists means a alloc was not found while expecting it to exist
	ErrAllocNotExists = errors.New("alloc doesn't exist")
)

//GetAlloc returns a pool by its primary key
func GetAlloc(conf *conf.Conf, db DB, pk AllocPK) (alloc *Alloc, err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal keys map")
	}

	var out *dynamodb.GetItemOutput
	if out, err = db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(conf.AllocsTableName),
		Key:       ipk,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to get item")
	}

	if out.Item == nil {
		return nil, ErrAllocNotExists
	}

	alloc = &Alloc{}
	err = dynamodbattribute.UnmarshalMap(out.Item, alloc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal item")
	}

	return alloc, nil
}

//PutNewAlloc will put an alloc with the condition the pk doesn't exist yet
func PutNewAlloc(conf *conf.Conf, db DB, alloc *Alloc) (err error) {
	item, err := dynamodbattribute.MarshalMap(alloc)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = db.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(conf.AllocsTableName),
		ConditionExpression: aws.String("attribute_not_exists(alloc)"),
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

//UpdateAllocTTL under the condition that it exists
func UpdateAllocTTL(conf *conf.Conf, db DB, ttl int64, apk AllocPK) (err error) {
	pk, err := dynamodbattribute.MarshalMap(apk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	ttlattr, err := dynamodbattribute.Marshal(ttl)
	if err != nil {
		return errors.Wrap(err, "failed to marshal new ttl")
	}

	if _, err = db.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:           aws.String(conf.AllocsTableName),
		Key:                 pk,
		UpdateExpression:    aws.String("SET #ttl = :ttl"),
		ConditionExpression: aws.String("attribute_exists(alloc)"),
		ExpressionAttributeNames: map[string]*string{
			"#ttl": aws.String("ttl"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":ttl": ttlattr,
		},
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to update item")
		}

		return ErrAllocNotExists
	}

	return nil
}
