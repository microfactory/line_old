package line

//ReplicaPK describes the replicas PK in the base tample
type ReplicaPK struct {
	DatasetID    string `dynamodbav:"set"`
	PoolWorkerID string `dynamodbav:"pwrk"`
}

//Replica represents the clone of a dataset available on a certain worker
type Replica struct {
	ReplicaPK
}
