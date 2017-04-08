package main

import (
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kelseyhightower/envconfig"
	"github.com/microfactory/line/line/actor"
	"github.com/microfactory/line/line/client"
	"github.com/microfactory/line/line/conf"
	"github.com/pkg/errors"
)

func testconf(tb testing.TB) (cfg *conf.Conf) {
	cfg = &conf.Conf{}
	err := envconfig.Process("LINE", cfg)
	if err != nil {
		tb.Fatal("failed to process env config", zap.Error(err))
	}

	return cfg
}

func awssess(tb testing.TB, conf *conf.Conf) (sess *session.Session) {
	var err error
	if sess, err = session.NewSession(
		&aws.Config{
			Region: aws.String(conf.AWSRegion),
			Credentials: credentials.NewStaticCredentials(
				conf.AWSAccessKeyID,
				conf.AWSSecretAccessKey,
				"",
			),
		},
	); err != nil {
		tb.Fatal("failed to setup aws session", zap.Error(err))
	}
	return sess
}

func endpoint(tb testing.TB) string {
	ep := os.Getenv("TEST_ENDPOINT")
	if ep == "" {
		tb.Fatal("env variable TEST_ENDPOINT was empty")
	}

	return ep
}

func nextalloc(c *client.Client, queueURL string) (alloc *client.Alloc, err error) {
	for i := 0; i < 5; i++ {
		out, err := c.ReceiveAllocs(&client.ReceiveAllocsInput{
			WorkerQueueURL:      queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
		})

		if err != nil {
			return nil, err
		}

		if len(out.Allocs) > 0 {
			alloc = out.Allocs[0]
			break
		}
	}

	if alloc == nil {
		return nil, errors.Errorf("didn't receive an alloc")
	}

	return alloc, nil
}

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

// func BenchmarkUserStory_1(b *testing.B) {
// 	ep := endpoint(b)
// 	conf := testconf(b)
// 	sess := awssess(b, conf)
//
// 	c, err := client.NewClient(ep, sess)
// 	ok(b, err)
//
// 	pool, err := c.CreatePool(&client.CreatePoolInput{})
// 	ok(b, err)
//
// 	worker, err := c.RegisterWorker(&client.RegisterWorkerInput{
// 		PoolID:   pool.PoolID,
// 		Capacity: 10,
// 	})
// 	ok(b, err)
//
// 	_, err = c.SendHeartbeat(&client.SendHeartbeatInput{
// 		PoolID:   pool.PoolID,
// 		WorkerID: worker.WorkerID,
// 		Datasets: []string{"d1"},
// 	})
// 	ok(b, err)
//
// 	_, err = c.ScheduleEval(&client.ScheduleEvalInput{
// 		PoolID:    pool.PoolID,
// 		Size:      3,
// 		DatasetID: "d1",
// 	})
// 	ok(b, err)
//
// 	b.Logf("scheduled eval, waiting for alloc...")
// 	alloc, err := nextalloc(c, worker.QueueURL)
// 	ok(b, err)
//
// 	_, err = c.SendHeartbeat(&client.SendHeartbeatInput{
// 		PoolID:   pool.PoolID,
// 		WorkerID: worker.WorkerID,
// 		Datasets: []string{"d1"},          //"I still have these datasets"
// 		Allocs:   []string{alloc.AllocID}, //"I still have these allocs running"
// 	})
// 	ok(b, err)
//
// 	_, err = c.CompleteAlloc(&client.CompleteAllocInput{
// 		PoolID:  pool.PoolID,
// 		AllocID: alloc.AllocID,
// 	})
// 	ok(b, err)
//
// 	b.Run("single worker tasks per second", func(b *testing.B) {
// 		for n := 0; n < b.N; n++ {
// 			_, err = c.ScheduleEval(&client.ScheduleEvalInput{
// 				PoolID: pool.PoolID,
// 				Size:   3,
// 			})
// 			ok(b, err)
//
// 			alloc, err := nextalloc(c, worker.QueueURL)
// 			ok(b, err)
//
// 			_, err = c.CompleteAlloc(&client.CompleteAllocInput{
// 				PoolID:  pool.PoolID,
// 				AllocID: alloc.AllocID,
// 			})
// 			ok(b, err)
// 		}
// 	})
//
// 	b.Run("10 workers", func(b *testing.B) {
// 		completedCh := make(chan *client.Alloc)
// 		completeFn := func(w *client.RegisterWorkerOutput) {
// 			for {
// 				alloc, err := nextalloc(c, w.QueueURL)
// 				if err != nil {
// 					break
// 				}
//
// 				_, err = c.SendHeartbeat(&client.SendHeartbeatInput{
// 					PoolID:   pool.PoolID,
// 					WorkerID: w.WorkerID,
// 					Allocs:   []string{alloc.AllocID},
// 				})
// 				ok(b, err)
//
// 				_, err = c.CompleteAlloc(&client.CompleteAllocInput{
// 					PoolID:  pool.PoolID,
// 					AllocID: alloc.AllocID,
// 				})
// 				ok(b, err)
//
// 				fmt.Println("p", pool.PoolID, "w", w.WorkerID, "a", alloc.AllocID, err)
// 				completedCh <- alloc
// 				ok(b, err)
// 			}
// 		}
//
// 		go completeFn(worker) //the worker we already have
// 		for n := 0; n < 9; n++ {
// 			worker, err := c.RegisterWorker(&client.RegisterWorkerInput{
// 				PoolID:   pool.PoolID,
// 				Capacity: 10,
// 			})
// 			ok(b, err)
//
// 			b.Logf("registered worker %d", n)
// 			go completeFn(worker) //nine new workers
// 		}
//
// 		numEvals := 100
// 		for n := 0; n < b.N; n++ {
// 			for i := 0; i < numEvals; i++ {
// 				_, err = c.ScheduleEval(&client.ScheduleEvalInput{
// 					PoolID: pool.PoolID,
// 					Size:   3,
// 				})
// 				ok(b, err)
//
// 				b.Logf("scheduled eval %d", i)
// 			}
//
// 			res := 0
// 			for range completedCh {
// 				b.Logf("completed alloc %d", res)
// 				res = res + 1
// 				if res >= numEvals {
// 					break
// 				}
// 			}
// 		}
//
// 	})
//
// 	_, err = c.DisbandPool(&client.DisbandPoolInput{
// 		PoolID: pool.PoolID,
// 	})
// 	ok(b, err)
// }
