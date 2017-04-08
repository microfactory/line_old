package actor

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/pkg/errors"
)

//Scheduler takes evals and turns them into allocs
type Scheduler interface {
	Schedule(eval *Eval) (err error)
}

//NopScheduler is a scheduler that does nothing
type NopScheduler struct{}

//Schedule implementation that does nothing
func (s *NopScheduler) Schedule(eval *Eval) (err error) {
	return nil
}

//SQS is our alias for the sqs connection
type SQS sqsiface.SQSAPI

//DB is our alias for the dynamodb connection
type DB dynamodbiface.DynamoDBAPI

//GenEntityID generates an reasonably unique value
func GenEntityID() (string, error) {
	idb := make([]byte, 10)
	_, err := rand.Read(idb)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate random id bytes")
	}

	return hex.EncodeToString(idb), nil
}
