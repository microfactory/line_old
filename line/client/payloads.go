package client

//CreatePoolInput is input to the pool creation call
type CreatePoolInput struct{}

//CreatePoolOutput is output of the pool creation call
type CreatePoolOutput struct {
	PoolID string `json:"pool_id"`
}

//CreateWorkerInput will create a worker in a pool
type CreateWorkerInput struct {
	PoolID   string `json:"pool_id"`
	Capacity int    `json:"capacity"`
}

//CreateWorkerOutput is returned when a worker is added to a pool
type CreateWorkerOutput struct {
	PoolID   string `json:"pool_id"`
	WorkerID string `json:"worker_id"`
	QueueURL string `json:"queue_url"`
	Capacity int    `json:"capacity"`
}

//DeleteWorkerInput will remove a worker
type DeleteWorkerInput struct {
	PoolID   string `json:"pool_id"`
	WorkerID string `json:"worker_id"`
}

//DeleteWorkerOutput is returned when a worker is removed
type DeleteWorkerOutput struct{}

//DeletePoolInput will remove a worker
type DeletePoolInput struct {
	PoolID string `json:"pool_id"`
}

//DeletePoolOutput is returned when a worker is removed
type DeletePoolOutput struct{}

//SendHeartbeatInput is send when updating heartbeats
type SendHeartbeatInput struct {
	PoolID   string   `json:"pool_id"`
	WorkerID string   `json:"worker_id"`
	Allocs   []string `json:"allocs"`
	Datasets []string `json:"datasets"`
}

//SendHeartbeatOutput is returned when updating heartbeats
type SendHeartbeatOutput struct {
	//@TODO expired allocs(?), or itself(?)
}

//ScheduleEvalInput will block until allocations are available for the worker
type ScheduleEvalInput struct {
	PoolID    string `json:"pool_id"`
	DatasetID string `json:"dataset_id"`
	Size      int    `json:"size"`
}

//ScheduleEvalOutput is returned when new allocs are available
type ScheduleEvalOutput struct{}

//ReceiveAllocsInput will block until allocations are available for the worker
type ReceiveAllocsInput struct {
	WorkerQueueURL      string `json:"worker_queue_url"`
	MaxNumberOfMessages int64  `json:"max_number_of_messages"`
	WaitTimeSeconds     int64  `json:"wait_time_seconds"`
}

//Alloc payload is returned to indicate an allocation
type Alloc struct {
	PoolID   string `json:"pool_id"`
	AllocID  string `json:"alloc_id"`
	WorkerID string `json:"worker_id"`
	//@TODO add some fields the worker has use for
}

//ReceiveAllocsOutput is returned when new allocs are available
type ReceiveAllocsOutput struct {
	Allocs []*Alloc `json:"allocs"`
}
