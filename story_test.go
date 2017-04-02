package main

import (
	"os"
	"testing"

	"github.com/microfactory/line/line/client"
)

func endpoint(t *testing.T) string {
	ep := os.Getenv("TEST_ENDPOINT")
	if ep == "" {
		t.Fatal("env variable TEST_ENDPOINT was empty")
	}

	return ep
}

func TestUserStory_1(t *testing.T) {
	ep := endpoint(t)
	c, err := client.NewClient(ep)
	ok(t, err)

	pool, err := c.CreatePool(&client.CreatePoolInput{})
	ok(t, err)

	worker, err := c.CreateWorker(&client.CreateWorkerInput{
		PoolID:   pool.PoolID,
		Capacity: 10,
	})
	ok(t, err)

	expired, err := c.SendHeartbeat(&client.SendHeartbeatInput{})
	ok(t, err)

	_, err = c.DeleteWorker(&client.DeleteWorkerInput{
		PoolID:   pool.PoolID,
		WorkerID: worker.WorkerID,
	})

	ok(t, err)
	_, err = c.DeletePool(&client.DeletePoolInput{
		PoolID: pool.PoolID,
	})

	ok(t, err)
	_ = expired
}
