package main

import (
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kelseyhightower/envconfig"
	"github.com/microfactory/line/line"
	"github.com/microfactory/line/line/client"
	"github.com/pkg/errors"
)

func testconf(tb testing.TB) (conf *line.Conf) {
	conf = &line.Conf{}
	err := envconfig.Process("LINE", conf)
	if err != nil {
		tb.Fatal("failed to process env config", zap.Error(err))
	}

	return conf
}

func awssess(tb testing.TB, conf *line.Conf) (sess *session.Session) {
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

func BenchmarkUserStory_1(b *testing.B) {
	ep := endpoint(b)
	conf := testconf(b)
	sess := awssess(b, conf)

	c, err := client.NewClient(ep, sess)
	ok(b, err)

	pool, err := c.CreatePool(&client.CreatePoolInput{})
	ok(b, err)

	worker, err := c.RegisterWorker(&client.RegisterWorkerInput{
		PoolID:   pool.PoolID,
		Capacity: 10,
	})
	ok(b, err)

	_, err = c.SendHeartbeat(&client.SendHeartbeatInput{
		PoolID:   pool.PoolID,
		WorkerID: worker.WorkerID,
		Datasets: []string{"d1"},
	})
	ok(b, err)

	_, err = c.ScheduleEval(&client.ScheduleEvalInput{
		PoolID:    pool.PoolID,
		Size:      3,
		DatasetID: "d1",
	})
	ok(b, err)

	b.Logf("scheduled eval, waiting for alloc...")
	alloc, err := nextalloc(c, worker.QueueURL)
	ok(b, err)

	_, err = c.SendHeartbeat(&client.SendHeartbeatInput{
		PoolID:   pool.PoolID,
		WorkerID: worker.WorkerID,
		Datasets: []string{"d1"},          //"I still have these datasets"
		Allocs:   []string{alloc.AllocID}, //"I still have these allocs running"
	})
	ok(b, err)

	_, err = c.CompleteAlloc(&client.CompleteAllocInput{
		PoolID:  pool.PoolID,
		AllocID: alloc.AllocID,
	})
	ok(b, err)

	b.Run("evals", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			_, err = c.ScheduleEval(&client.ScheduleEvalInput{
				PoolID: pool.PoolID,
				Size:   3,
			})
			ok(b, err)

			alloc, err := nextalloc(c, worker.QueueURL)
			ok(b, err)

			_, err = c.CompleteAlloc(&client.CompleteAllocInput{
				PoolID:  pool.PoolID,
				AllocID: alloc.AllocID,
			})
			ok(b, err)
		}
	})

	_, err = c.DisbandPool(&client.DisbandPoolInput{
		PoolID: pool.PoolID,
	})
	ok(b, err)
}
