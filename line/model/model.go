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
func query(db DB, tname, iname string, next func() interface{}, proj *Exp, filt *Exp, kcond *Exp, limit int64, maxPages int) (err error) {
	if maxPages == 0 {
		maxPages = 1
	}

	inp := &dynamodb.QueryInput{
		TableName: aws.String(tname),
	}

	if iname != "" {
		inp.SetIndexName(iname)
	}

	if kcond == nil {
		return errors.Errorf("must provide a key condition expression")
	}

	inp.KeyConditionExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = kcond.Get()
	if err != nil {
		return errors.Wrap(err, "error in key condition expression")
	}

	if proj != nil {
		inp.ProjectionExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = proj.GetMerged(inp.ExpressionAttributeNames, inp.ExpressionAttributeValues)
		if err != nil {
			return errors.Wrap(err, "error in projection expression")
		}
	}

	if filt != nil {
		inp.FilterExpression, inp.ExpressionAttributeNames, inp.ExpressionAttributeValues, err = filt.GetMerged(inp.ExpressionAttributeNames, inp.ExpressionAttributeValues)
		if err != nil {
			return errors.Wrap(err, "error in filter expression")
		}
	}

	if limit != 0 {
		inp.SetLimit(limit)
	}

	pageNum := 0
	var lastErr error
	if err = db.QueryPages(inp,
		func(out *dynamodb.QueryOutput, lastPage bool) bool {
			pageNum++
			for _, item := range out.Items {
				next := next()
				err := dynamodbattribute.UnmarshalMap(item, next)
				if err != nil {
					lastErr = errors.Wrap(err, "failed to unmarshal item")
					return false
				}
			}

			return pageNum < maxPages
		}); err != nil {
		return errors.Wrap(err, "failed to query")
	}

	if lastErr != nil {
		return lastErr
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
