package model

//PoolPK is the primary key of a pool
type PoolPK struct {
	PoolID string
}

//Pool is the combined capacity of workers
type Pool struct {
	PoolPK
}

//PoolTable provides persistence for pools
type PoolTable struct{}
