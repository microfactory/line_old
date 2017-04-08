package line

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/pkg/errors"
)

//ReplicaPK describes the replicas PK in the base tample
type ReplicaPK struct {
	PoolID    string `dynamodbav:"pool"`
	ReplicaID string `dynamodbav:"rpl"`
}

//Replica represents the clone of a dataset available on a certain worker
type Replica struct {
	ReplicaPK
	TTL int64 `dynamodbav:"ttl"`
}

//PutReplica will put an replica with the condition the pk doesn't exist yet
func PutReplica(conf *Conf, db DB, replica *Replica) (err error) {
	item, err := dynamodbattribute.MarshalMap(replica)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(conf.ReplicasTableName),
		Item:      item,
	}); err != nil {
		return err
	}

	return nil
}

//DeleteReplica deletes a replica by pk
func DeleteReplica(conf *Conf, db DB, pk ReplicaPK) (err error) {
	ipk, err := dynamodbattribute.MarshalMap(pk)
	if err != nil {
		return errors.Wrap(err, "failed to marshal keys map")
	}

	if _, err = db.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(conf.ReplicasTableName),
		Key:       ipk,
	}); err != nil {
		return err
	}

	return nil
}
