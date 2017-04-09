package model

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/pkg/errors"
)

func TestExp(t *testing.T) {
	exp, names, vals, err := NewExp("attribute_not_exists(#wrk)").
		Name("#wrk", "wrk").
		Value(":foo", "bar").
		Get()

	ok(t, err)
	equals(t, "attribute_not_exists(#wrk)", aws.StringValue(exp))
	assert(t, len(names) == 1, "should have one name, got: %#v", names)
	assert(t, len(vals) == 1, "should have one value, got: %#v", vals)

	equals(t, "wrk", aws.StringValue(names["#wrk"]))
	equals(t, "bar", aws.StringValue(vals[":foo"].S))
}

func TestWorkerCRUD(t *testing.T) {
	cfg, err := ConfFromEnv()
	ok(t, err)
	db := dynamodb.New(awssess(t, cfg))
	wt := NewWorkerTable(db, cfg)

	_, err = wt.Get(WorkerPK{"bo", "gus"})
	assert(t, errors.Cause(err) == ErrWorkerNotExists, "should receive error indicating it doesn't exist anymore, got: %+v", err)

	w1 := &Worker{WorkerPK{"p1", "w1"}, 10}
	err = wt.Put(w1)
	ok(t, err)

	err = wt.PutNew(w1)
	assert(t, errors.Cause(err) == ErrWorkerExists, "should receive error indicating it exists already, got: %+v", err)

	w2, err := wt.Get(w1.WorkerPK)
	ok(t, err)
	assert(t, w2.PoolID == w1.PoolID, "should equal")
	assert(t, w2.WorkerID == w1.WorkerID, "should equal")
	assert(t, w2.Capacity == w1.Capacity, "should equal")

	err = wt.Update(w2.WorkerPK, NewExp("SET cap = :cap").Value(":cap", 12))
	ok(t, err)

	err = wt.UpdateExisting(WorkerPK{"bo", "gus"}, NewExp("SET cap = :cap").Value(":cap", 12))
	assert(t, errors.Cause(err) == ErrWorkerNotExists, "should receive error indicating it doesn't exist anymore, got: %+v", err)

	w3, err := wt.Get(w1.WorkerPK)
	assert(t, w3.Capacity == 12, "capacity should have been updated")
	ok(t, err)

	w4, err := wt.GetMin(w1.WorkerPK)
	ok(t, err)
	assert(t, w4.Capacity == 0, "capacity should not havee been projected, got: %+v", w4.Capacity)
	ok(t, err)

	err = wt.Delete(w2.WorkerPK)
	ok(t, err)

	err = wt.Delete(w2.WorkerPK)
	ok(t, err)

	err = wt.DeleteExisting(w2.WorkerPK)
	assert(t, errors.Cause(err) == ErrWorkerNotExists, "should receive error indicating it doesn't exist anymore, got: %+v", err)
}

func TestWorkerQuery(t *testing.T) {
	cfg, err := ConfFromEnv()
	ok(t, err)
	db := dynamodb.New(awssess(t, cfg))
	wt := NewWorkerTable(db, cfg)

	w1 := &Worker{WorkerPK{"p1", "w1"}, 1}
	err = wt.Put(w1)
	ok(t, err)
	defer func() {
		err = wt.Delete(w1.WorkerPK)
		ok(t, err)
	}()

	w2 := &Worker{WorkerPK{"p1", "w2"}, 3}
	err = wt.Put(w2)
	ok(t, err)
	defer func() {
		err = wt.Delete(w2.WorkerPK)
		ok(t, err)
	}()

	w3 := &Worker{WorkerPK{"p1", "w3"}, 10}
	err = wt.Put(w3)
	ok(t, err)
	defer func() {
		err = wt.Delete(w3.WorkerPK)
		ok(t, err)
	}()

	t.Run("queries", func(t *testing.T) {
		l1, err := wt.Query(PoolPK{"p1"})
		ok(t, err)
		assert(t, len(l1) == 3, "expected 3 workers, got: %+v", l1)
		equals(t, w1.WorkerID, l1[0].WorkerID)
		equals(t, w1.Capacity, l1[0].Capacity)
	})
}
