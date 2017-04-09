package model

import "errors"

var (
	//ErrWorkerExists is returned when a conditional for existence failed
	ErrWorkerExists = errors.New("worker already exists")

	//ErrWorkerNotExists is returned when an item is not available in the database
	ErrWorkerNotExists = errors.New("worker doesn't exist")
)

//WorkerPK is the workers primary key
type WorkerPK struct {
	PoolID   string `dynamodbav:"pool"`
	WorkerID string `dynamodbav:"wrk"`
}

//Worker holds capacity for a pool
type Worker struct {
	WorkerPK
	Capacity int64 `dynamodbav:"cap"`
}

//WorkerTable provides persistence for workers
type WorkerTable struct {
	db  DB
	cfg *Conf
}

//NewWorkerTable sets up a new worker table
func NewWorkerTable(db DB, cfg *Conf) *WorkerTable {
	return &WorkerTable{db, cfg}
}

//Get a worker from the base table
func (t *WorkerTable) Get(pk WorkerPK) (i *Worker, err error) {
	i = &Worker{}
	return i, get(t.db, t.cfg.WorkersTableName, pk, i, ErrWorkerNotExists)
}

//Put a worker in the base table
func (t *WorkerTable) Put(i *Worker) (err error) {
	return put(t.db, t.cfg.WorkersTableName, i, nil, nil)
}

//Update a worker with the provided pk
func (t *WorkerTable) Update(pk WorkerPK, exp *Exp) (err error) {
	return update(t.db, t.cfg.WorkersTableName, pk, exp, nil, nil)
}

//UpdateExisting an existing worker with the provided pk
func (t *WorkerTable) UpdateExisting(pk WorkerPK, exp *Exp) (err error) {
	return update(t.db, t.cfg.WorkersTableName, pk, exp, NewExp("attribute_exists(wrk)"), ErrWorkerNotExists)
}

//PutNew inserts a worker on the condition it doesn't exist yet
func (t *WorkerTable) PutNew(i *Worker) (err error) {
	return put(t.db, t.cfg.WorkersTableName, i, NewExp("attribute_not_exists(#wrk)").Name("#wrk", "wrk"), ErrWorkerExists)
}

//Delete a worker from the base table
func (t *WorkerTable) Delete(pk WorkerPK) (err error) {
	return delete(t.db, t.cfg.WorkersTableName, pk, nil, nil)
}

//DeleteExisting a worker on the condition it exists\
func (t *WorkerTable) DeleteExisting(pk WorkerPK) (err error) {
	return delete(t.db, t.cfg.WorkersTableName, pk, NewExp("attribute_exists(#wrk)").Name("#wrk", "wrk"), ErrWorkerNotExists)
}
