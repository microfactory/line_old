package actor_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/microfactory/line/line/actor"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func TestWorkerActorCRUD(t *testing.T) {
	logs, err := zap.NewProduction()
	ok(t, err)

	conf := testconf(t)
	sess := awssess(t, conf)

	pmgr := actor.NewPoolManager(
		actor.NewPoolManagerConf(conf),
		sqs.New(sess),
		dynamodb.New(sess),
		logs,
	)

	pool, err := pmgr.CreatePool()
	ok(t, err)
	defer func() {
		err = pool.Disband()
		ok(t, err)
	}()

	wmgr := actor.NewWorkers(
		actor.NewWorkersConf(conf),
		sqs.New(sess),
		dynamodb.New(sess), logs, pool.PoolPK)

	worker1, err := wmgr.CreateWorker(10, pool.PoolPK)
	ok(t, err)
	assert(t, (worker1 != nil), "%#v", worker1)
	assert(t, (worker1.Capacity == 10), "%#v", worker1)
	assert(t, (worker1.PoolID != ""), "%#v", worker1)
	assert(t, (worker1.WorkerID != ""), "%#v", worker1)
	assert(t, (worker1.QueueURL != ""), "%#v", worker1)
	assert(t, (worker1.TTL != 0), "%#v", worker1)

	worker2, err := wmgr.FetchWorker(worker1.WorkerPK)
	ok(t, err)
	assert(t, (worker2 != nil), "%#v", worker2)
	assert(t, (worker2.Capacity == worker1.Capacity), "%#v %#v", worker2, worker1)
	assert(t, (worker2.PoolID == worker1.PoolID), "%#v %#v", worker2, worker1)
	assert(t, (worker2.QueueURL == worker1.QueueURL), "%#v %#v", worker2, worker1)
	assert(t, (worker2.TTL == worker1.TTL), "%#v %#v", worker2, worker1)

	err = wmgr.Delete(worker2.WorkerPK)
	ok(t, err)

	err = wmgr.Delete(worker2.WorkerPK)
	assert(t, actor.ErrWorkerNotExists == errors.Cause(err), "should give not existing err, got: %#v", err)

	_, err = wmgr.FetchWorker(worker1.WorkerPK)
	assert(t, actor.ErrWorkerNotExists == errors.Cause(err), "should give not existing err, got: %#v", err)
}

func TestWorkerCapacityOperations(t *testing.T) {
	logs, err := zap.NewProduction()
	ok(t, err)

	conf := testconf(t)
	sess := awssess(t, conf)
	pmgr := actor.NewPoolManager(
		actor.NewPoolManagerConf(conf),
		sqs.New(sess),
		dynamodb.New(sess),
		logs,
	)

	pool, err := pmgr.CreatePool()
	ok(t, err)
	defer func() {
		err = pool.Disband()
		ok(t, err)
	}()

	wmgr := actor.NewWorkers(
		actor.NewWorkersConf(conf),
		sqs.New(sess),
		dynamodb.New(sess), logs, pool.PoolPK)

	worker1, err := wmgr.CreateWorker(10, pool.PoolPK)
	ok(t, err)

	worker2, err := wmgr.CreateWorker(1, pool.PoolPK)
	ok(t, err)

	defer func() {
		err = wmgr.Delete(worker1.WorkerPK)
		ok(t, err)
		err = wmgr.Delete(worker2.WorkerPK)
		ok(t, err)
	}()

	t.Run("work with capacity", func(t *testing.T) {
		workers, err := wmgr.ListWithCapacity(pool.PoolPK, 3)
		ok(t, err)
		assert(t, len(workers) == 1, "expected one worker with enough capacity, got %d", len(workers))
		assert(t, workers[0].WorkerID == worker1.WorkerID, "expected worker id to equal %s", worker1.WorkerID)

		err = wmgr.SubtractCapacity(worker1.WorkerPK, 7)
		ok(t, err)

		err = wmgr.SubtractCapacity(worker1.WorkerPK, 7)
		assert(t, errors.Cause(err) == actor.ErrWorkerNotEnoughCapacity, "expected error indicating not enough capacity, got: %v", err)
	})
}
