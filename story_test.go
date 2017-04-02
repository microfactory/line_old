package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func turl(t *testing.T, path string) string {
	ep := os.Getenv("TEST_ENDPOINT")
	if ep == "" {
		t.Fatal("env variable TEST_ENDPOINT was empty")
	}

	return fmt.Sprintf("%s/%s", ep, strings.TrimLeft(path, "/"))
}

func CreateNewProject(t *testing.T) interface{} {
	//setup a new project
	return nil
}

func CreateTaskInProject(t *testing.T, input interface{}, project interface{}) interface{} {
	//create a new task under the project with the provided dataset as input
	return nil
}

func CreateNewPool(t *testing.T) interface{} {
	//create a new pool
	return nil
}

func ScheduleTaskOntoPool(t *testing.T, task interface{}, pool interface{}) {
	//take a task and ask the pool to schedule it
}

func StartWorkerForPool(t *testing.T, pool interface{}) interface{} {
	//present new capacity to the pool and open a new message queue
	return nil
}

func SendWorkerHeartbeat(t *testing.T, pool interface{}) interface{} {
	//send heartbeat to the worker endpoint, indicating the worker is still active
	return nil
}

func WaitForAllocation(t *testing.T, worker interface{}) interface{} {
	//wait for an allocation to arrive on the workers queue
	return nil
}

func SendAllocHeartbeat(t *testing.T, alloc interface{}) interface{} {
	//send a new heartbeat the allocations endpoint, adding to the ttl
	return nil
}

func CompleteAlloc(t *testing.T, alloc interface{}) {
	//complete an allocation by sending the resulting dataset to the server
}

func CreateInputDataset(t *testing.T) interface{} {
	//create a new dataset for uploading input
	return nil
}

func RetrieveTaskOutput(t *testing.T, task interface{}) interface{} {
	//get a tasks output dataset
	return nil
}

func UploadInputAsFiles(t *testing.T, input interface{}) {
	//stream files into tar archive and and upload chunks
}

func DownloadOutputAsFiles(t *testing.T, output interface{}) {
	//read dataset index object and fetch chunks listed under the header
}

func SendAllocLogs(t *testing.T, alloc interface{}) {
	//interact with containerd and send log lines to AWS kinesis stream
}

func SendAllocMetrics(t *testing.T, metris interface{}) {
	//interact with containerd metrics and send them over to an AWS kinesis stream
}

func RetrieveTaskLogs(t *testing.T, task interface{}) interface{} {
	//fetches aggregated log lines from s3 under firehose directory structure
	return nil
}

func RetrieveTaskMetrics(t *testing.T, task interface{}) interface{} {
	//fetches aggegrate metrics from s3 under firehose directory structure
	return nil
}

//interesting user story ideas:
// - cancel a task after it has been scheduled
// - system containers that run on every worker

// clone a bare git repository
// report replica
// schedule task with input

func TestUserStory_1(t *testing.T) {
	pool := CreateNewPool(t)
	project := CreateNewProject(t)

	input := CreateInputDataset(t)
	UploadInputAsFiles(t, input)

	task := CreateTaskInProject(t, input, project)

	ScheduleTaskOntoPool(t, task, pool) //
	worker := StartWorkerForPool(t, pool)
	SendWorkerHeartbeat(t, worker)

	alloc := WaitForAllocation(t, worker)
	SendAllocHeartbeat(t, alloc)
	SendAllocLogs(t, alloc)    //alloc contains kinesis stream
	SendAllocMetrics(t, alloc) //alloc contains kinesis stream endpoint
	CompleteAlloc(t, alloc)

	logs := RetrieveTaskLogs(t, task)
	metrics := RetrieveTaskMetrics(t, task)
	output := RetrieveTaskOutput(t, task)

	DownloadOutputAsFiles(t, output)

	_ = logs
	_ = metrics
}
