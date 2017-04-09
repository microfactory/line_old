package actor

//WorkerPK uniquely identifies a worker
type WorkerPK struct {
	PoolID   string `dynamodbav:"pool"`
	WorkerID string `dynamodbav:"wrk"`
}

//Worker represents a source of capacity
type Worker struct {
	WorkerPK
	Capacity int64  `dynamodbav:"cap"`
	QueueURL string `dynamodbav:"que"`
	TTL      int64  `dynamodbav:"ttl"`
}
