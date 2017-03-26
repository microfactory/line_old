package line

//WorkerPK uniquely identifies a worker
type WorkerPK struct {
	PoolID   string `dynamodbav:"pool"`
	WorkerID string `dynamodbav:"wrk"`
}

//Worker represents a source of capacity
type Worker struct {
	WorkerPK
	Capacity int `dynamodbav:"cap"`
}
