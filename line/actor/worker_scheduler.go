package actor

import "fmt"

//WorkerScheduler implements the scheduler interface
type WorkerScheduler struct {
	pool PoolPK
}

//Schedule evals on the pool
func (ws *WorkerScheduler) Schedule(eval *Eval) (err error) {
	fmt.Println(eval)
	return nil
}
