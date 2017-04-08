package actor_test

import (
	"testing"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/microfactory/line/line/actor"
	"github.com/pkg/errors"
)

func TestPoolActorCRUD(t *testing.T) {
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

	pool1, err := pmgr.CreatePool()
	ok(t, err)
	assert(t, (pool1 != nil), "%#v", pool1)
	assert(t, (pool1.PoolID != ""), "%#v", pool1)
	assert(t, (pool1.QueueURL != ""), "%#v", pool1)
	assert(t, (pool1.TTL != 0), "%#v", pool1)

	pool2, err := pmgr.FetchPool(pool1.PoolPK)
	ok(t, err)
	assert(t, (pool2 != nil), "%#v", pool2)
	assert(t, (pool2.PoolID == pool1.PoolID), "%#v %#v", pool2, pool1)
	assert(t, (pool2.QueueURL == pool1.QueueURL), "%#v %#v", pool2, pool1)
	assert(t, (pool2.TTL == pool1.TTL), "%#v %#v", pool2, pool1)

	err = pool2.Disband()
	ok(t, err)

	err = pool2.Disband()
	assert(t, errors.Cause(err) == actor.ErrPoolNotExists, "disbanded pool should not exist, got: %#v", err)

	_, err = pmgr.FetchPool(pool2.PoolPK)
	assert(t, err == actor.ErrPoolNotExists, "disbanded pool should not exist, got: %#v", err)
}

func TestPoolEvalHandling(t *testing.T) {
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

	t.Run("schedule_noop", func(t *testing.T) {
		doneCh := make(chan struct{})
		go pool.HandleEvals(&actor.NopScheduler{}, 1, doneCh)
		for i := 0; i < 25; i++ {
			err = pool.ScheduleEval(&actor.Eval{})
			ok(t, err)
		}

		close(doneCh)
	})

	t.Run("schedule_workers", func(t *testing.T) {
		wmgr := actor.NewWorkerManager(
			actor.NewWorkerManagerConf(conf),
			sqs.New(sess),
			dynamodb.New(sess), logs, pool.PoolPK)

		doneCh := make(chan struct{})
		go pool.HandleEvals(wmgr, 1, doneCh)
		for i := 0; i < 25; i++ {
			err = pool.ScheduleEval(&actor.Eval{})
			ok(t, err)
		}

		close(doneCh)
	})

}
