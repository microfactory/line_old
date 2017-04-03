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

func endpoint(t *testing.T) string {
	ep := os.Getenv("TEST_ENDPOINT")
	if ep == "" {
		t.Fatal("env variable TEST_ENDPOINT was empty")
	}

	return ep
}

func TestUserStory_1(t *testing.T) {
	ep := endpoint(t)
	conf := testconf(t)
	sess := awssess(t, conf)

	c, err := client.NewClient(ep, sess)
	ok(t, err)

	pool, err := c.CreatePool(&client.CreatePoolInput{})
	ok(t, err)

	worker, err := c.CreateWorker(&client.CreateWorkerInput{
		PoolID:   pool.PoolID,
		Capacity: 10,
	})
	ok(t, err)

	_, err = c.SendHeartbeat(&client.SendHeartbeatInput{
		PoolID:   pool.PoolID,
		WorkerID: worker.WorkerID,
		Datasets: []string{"d1"},
	})
	ok(t, err)

	_, err = c.ScheduleEval(&client.ScheduleEvalInput{
		PoolID:    pool.PoolID,
		Size:      3,
		DatasetID: "d1",
	})
	ok(t, err)

	allocs := []*client.Alloc{}
	for i := 0; i < 5; i++ {
		recv, err := c.ReceiveAllocs(&client.ReceiveAllocsInput{
			WorkerQueueURL:      worker.QueueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
		})
		ok(t, err)

		if len(recv.Allocs) > 0 {
			allocs = recv.Allocs
			break
		}
	}

	assert(t, len(allocs) > 0, "expected to have received some allocs", allocs)

	//@TODO send heartbeat with the allocs

	_, err = c.DeleteWorker(&client.DeleteWorkerInput{
		PoolID:   pool.PoolID,
		WorkerID: worker.WorkerID,
	})
	ok(t, err)

	_, err = c.DeletePool(&client.DeletePoolInput{
		PoolID: pool.PoolID,
	})
	ok(t, err)
}
