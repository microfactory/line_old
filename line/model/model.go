package model

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/pkg/errors"
)

//DB is our local alias for the dynamo interface
type DB dynamodbiface.DynamoDBAPI

var (
	//ErrItemExists is returned when a conditional for existence failed
	ErrItemExists = errors.New("item already exists")

	//ErrItemNotExists is returned when an item is not available in the database
	ErrItemNotExists = errors.New("item doesn't exist")
)

// delete an item from the table by primary key
func delete(db DB, tname string, pk interface{}, existAttr string) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal primary key")
	}

	inp := &dynamodb.DeleteItemInput{
		TableName: aws.String(tname),
		Key:       ipk,
	}

	if existAttr != "" {
		inp.ConditionExpression = aws.String(
			"attribute_exists(#" + existAttr + ")",
		)
		inp.ExpressionAttributeNames = map[string]*string{
			"#" + existAttr: aws.String(existAttr),
		}
	}

	if _, err = db.DeleteItem(inp); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to delete item")
		}

		return ErrItemNotExists
	}

	return nil
}

// get will attempt to get an item by pk and deserialize into item
func get(db DB, tname string, pk interface{}, item interface{}) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal primary key")
	}

	var out *dynamodb.GetItemOutput
	if out, err = db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tname),
		Key:       ipk,
	}); err != nil {
		return errors.Wrap(err, "failed to get item")
	}

	if out.Item == nil {
		return ErrItemNotExists
	}

	err = dynamodbattribute.UnmarshalMap(out.Item, item)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal item")
	}

	return nil
}

// put an item into dynamodb in the provided table
func put(db DB, tname string, item interface{}, existAttr string) (err error) {
	it, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	inp := &dynamodb.PutItemInput{
		TableName: aws.String(tname),
		Item:      it,
	}

	if existAttr != "" {
		inp.ConditionExpression = aws.String(
			"attribute_not_exists(#" + existAttr + ")",
		)
		inp.ExpressionAttributeNames = map[string]*string{
			"#" + existAttr: aws.String(existAttr),
		}
	}

	if _, err = db.PutItem(inp); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		return ErrItemExists
	}

	return nil
}
