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

func CreateNewPool(t *testing.T) interface{} {
	return nil
}

func ScheduleTaskOntoPool(t *testing.T, task interface{}, pool interface{}) {

}

func StartWorkerForPool(t *testing.T, pool interface{}) interface{} {
	return nil
}

func SendWorkerHeartbeat(t *testing.T, pool interface{}) interface{} {
	return nil
}

func WaitForAllocation(t *testing.T, worker interface{}) interface{} {
	return nil
}

func SendAllocHeartbeat(t *testing.T, alloc interface{}) interface{} {
	return nil
}

func CreateNewProject(t *testing.T) interface{} {
	return nil
}

func CreateTaskInProject(t *testing.T, input interface{}, project interface{}) interface{} {
	return nil
}

func CompleteAlloc(t *testing.T, alloc interface{}) {

}

func CreateInputDataset(t *testing.T) interface{} {
	return nil
}

func RetrieveTaskOutput(t *testing.T, task interface{}) interface{} {
	return nil
}

func UploadInputAsFiles(t *testing.T, input interface{}) {
}

func DownloadOutputAsFiles(t *testing.T, output interface{}) {

}

func SendAllocLogs(t *testing.T, alloc interface{}) {

}

func SendAllocMetrics(t *testing.T, metris interface{}) {

}

func RetrieveTaskLogs(t *testing.T, task interface{}) interface{} {
	return nil
}

func RetrieveTaskMetrics(t *testing.T, task interface{}) interface{} {
	return nil
}

func TestUserStory_1(t *testing.T) {
	pool := CreateNewPool(t)
	project := CreateNewProject(t)

	input := CreateInputDataset(t)
	UploadInputAsFiles(t, input)

	task := CreateTaskInProject(t, input, project)
	ScheduleTaskOntoPool(t, task, pool)

	worker := StartWorkerForPool(t, pool)
	SendWorkerHeartbeat(t, worker)

	alloc := WaitForAllocation(t, worker)
	SendAllocHeartbeat(t, alloc)
	SendAllocLogs(t, alloc)
	SendAllocMetrics(t, alloc)
	CompleteAlloc(t, alloc)

	logs := RetrieveTaskLogs(t, task)
	metrics := RetrieveTaskMetrics(t, task)
	output := RetrieveTaskOutput(t, task)

	DownloadOutputAsFiles(t, output)

	_ = logs
	_ = metrics
}
