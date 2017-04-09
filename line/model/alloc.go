package model

//AllocPK is the primary key of an alloction
type AllocPK struct {
	PoolID  string
	AllocID string
}

//Alloc is claimed capacity
type Alloc struct {
	AllocPK
}

//AllocTable provides persistence for allocations
type AllocTable struct{}
