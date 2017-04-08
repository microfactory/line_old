package actor_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/microfactory/line/line/actor"
)

func TestPoolActor(t *testing.T) {
	conf := testconf(t)
	sess := awssess(t, conf)

	pmgr := actor.NewPoolManager(
		actor.NewPoolManagerConf(conf),
		sqs.New(sess),
		dynamodb.New(sess),
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

	err = pmgr.DisbandPool(pool2.PoolPK)
	ok(t, err)

	_, err = pmgr.FetchPool(pool2.PoolPK)
	assert(t, err == actor.ErrPoolNotExists, "disbanded pool should not exist, got: %#v", err)
}
