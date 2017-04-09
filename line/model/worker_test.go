package model

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/pkg/errors"
)

func TestWorkerCRUD(t *testing.T) {
	cfg, err := ConfFromEnv()
	ok(t, err)
	db := dynamodb.New(awssess(t, cfg))
	wt := NewWorkerTable(db, cfg)

	w1 := &Worker{WorkerPK{"p1", "w1"}, 10}
	err = wt.Put(w1)
	ok(t, err)

	err = wt.PutNew(w1)
	assert(t, errors.Cause(err) == ErrItemExists, "should receive error indicating it exists already, got: %+v", err)

	w2, err := wt.Get(w1.WorkerPK)
	ok(t, err)
	assert(t, w2.PoolID == w1.PoolID, "should equal")
	assert(t, w2.WorkerID == w1.WorkerID, "should equal")
	assert(t, w2.Capacity == w1.Capacity, "should equal")

	err = wt.Delete(w2.WorkerPK)
	ok(t, err)

	err = wt.Delete(w2.WorkerPK)
	ok(t, err)

	err = wt.DeleteExisting(w2.WorkerPK)
	assert(t, errors.Cause(err) == ErrItemNotExists, "should receive error indicating it doesn't exist anymore, got: %+v", err)
}
