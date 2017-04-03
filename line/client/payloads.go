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

//ReceiveAllocsInput will block until allocations are available for the worker
type ReceiveAllocsInput struct{}

//ReceiveAllocsOutput is returned when new allocs are available
type ReceiveAllocsOutput struct{}

//ScheduleEvalInput will block until allocations are available for the worker
type ScheduleEvalInput struct{}

//ScheduleEvalOutput is returned when new allocs are available
type ScheduleEvalOutput struct{}
