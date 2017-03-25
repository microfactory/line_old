package line

import (
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

//TaskPK describes the task's primary key in the base table
type TaskPK struct {
	ProjectID string `dynamodbav:"prj"` //Base Partition Key
	TaskID    string `dynamodbav:"tsk"` //Base Sort Key
}

//Task represents a planned execution
type Task struct {
	TaskPK
	PoolID        string `dynamodbav:"pool"`
	ActivityToken string `dynamodbav:"-"`
}

var (
	//ErrTaskExists means a task exists while it was expected not to
	ErrTaskExists = errors.New("task already exists")

	//ErrTaskNotExists means a task was not found while expecting it to exist
	ErrTaskNotExists = errors.New("task doesn't exist")
)

//PutNewTask will put a pool with the condition the pk doesn't exist yet
func PutNewTask(conf *Conf, db DB, task *Task) (err error) {
	item, err := dynamodbattribute.MarshalMap(task)
	if err != nil {
		return errors.Wrap(err, "failed to marshal item map")
	}

	if _, err = db.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(conf.TasksTableName),
		ConditionExpression: aws.String("attribute_not_exists(tsk)"),
		Item:                item,
	}); err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != dynamodb.ErrCodeConditionalCheckFailedException {
			return errors.Wrap(err, "failed to put item")
		}

		return ErrTaskExists
	}

	return nil
}
