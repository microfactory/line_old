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

	_, err = c.SendHeartbeat(&client.SendHeartbeatInput{
		PoolID:   pool.PoolID,
		WorkerID: worker.WorkerID,
		Datasets: []string{"d1", "d2", "d3"},
	})
	ok(t, err)

	_, err = c.ScheduleEval(&client.ScheduleEvalInput{})
	ok(t, err)

	allocs, err := c.ReceiveAllocs(&client.ReceiveAllocsInput{})
	ok(t, err)
	_ = allocs //@TODO assert allocation to match eval values

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
