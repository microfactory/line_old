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

// query reads a list of items from dynamodb
func query(db DB, tname, idxname string, next func() interface{}, proj *Exp, filt *Exp, kcond *Exp) (err error) {

	inp := &dynamodb.QueryInput{
		//@TODO how to handle different indexes with different projections
		TableName: aws.String(tname),
	}

	if kcond == nil {
		return errors.Errorf("must provide a key condition expression")
	}

	inp.KeyConditionExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = kcond.Get()
	if err != nil {
		return errors.Wrap(err, "error in key condition expression")
	}

	//@TODO handle projection expr
	//@TODO handle filter expr

	var out *dynamodb.QueryOutput
	if out, err = db.Query(inp); err != nil {
		return errors.Wrap(err, "failed to query")
	}

	//@TODO how to handle pagination and limits
	for _, item := range out.Items {
		next := next()
		err := dynamodbattribute.UnmarshalMap(item, next)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal item")
		}
	}

	return nil
}

// delete an item from the table by primary key
func delete(db DB, tname string, pk interface{}, cond *Exp, condErr error) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal primary key")
	}

	inp := &dynamodb.DeleteItemInput{
		TableName: aws.String(tname),
		Key:       ipk,
	}

	if cond != nil {
		inp.ConditionExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = cond.Get()
		if err != nil {
			return errors.Wrap(err, "error in conditional expression")
		}
	}

	if _, err = db.DeleteItem(inp); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to delete item")
		}

		if condErr != nil {
			return condErr
		}
		return err
	}

	return nil
}

// update an item with primary key pk with exp
func update(db DB, tname string, pk interface{}, upd *Exp, cond *Exp, condErr error) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal primary key")
	}

	inp := &dynamodb.UpdateItemInput{
		Key:       ipk,
		TableName: aws.String(tname),
	}

	if upd != nil {
		inp.UpdateExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = upd.Get()
		if err != nil {
			return errors.Wrap(err, "error in update expression")
		}
	}

	if cond != nil {
		inp.ConditionExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = cond.GetMerged(inp.ExpressionAttributeNames, inp.ExpressionAttributeValues)
		if err != nil {
			return errors.Wrap(err, "error in conditional expression")
		}
	}

	if _, err = db.UpdateItem(inp); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		if condErr != nil {
			return condErr
		}

		return err
	}

	return nil
}

// get will attempt to get an item by pk and deserialize into item
func get(db DB, tname string, pk interface{}, item interface{}, proj *Exp, errItemNil error) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal primary key")
	}

	inp := &dynamodb.GetItemInput{
		TableName: aws.String(tname),
		Key:       ipk,
	}

	if proj != nil {
		inp.ProjectionExpression, inp.ExpressionAttributeNames, _, err = proj.Get()
		if err != nil {
			return errors.Wrap(err, "error in conditional expression")
		}
	}

	var out *dynamodb.GetItemOutput
	if out, err = db.GetItem(inp); err != nil {
		return errors.Wrap(err, "failed to get item")
	}

	if out.Item == nil {
		return errItemNil
	}

	err = dynamodbattribute.UnmarshalMap(out.Item, item)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal item")
	}

	return nil
}

// put an item into dynamodb in the provided table
func put(db DB, tname string, item interface{}, cond *Exp, condErr error) (err error) {
	it, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	inp := &dynamodb.PutItemInput{
		TableName: aws.String(tname),
		Item:      it,
	}

	if cond != nil {
		inp.ConditionExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = cond.Get()
		if err != nil {
			return errors.Wrap(err, "error in conditional expression")
		}
	}

	if _, err = db.PutItem(inp); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		if condErr != nil {
			return condErr
		}
		return err
	}

	return nil
}
