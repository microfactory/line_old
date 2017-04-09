package actor

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
)

//WorkerScheduler implements the scheduler interface
type WorkerScheduler struct {
	workers *Workers
	pool    PoolPK
}

//Schedule evals on the pool
func (ws *WorkerScheduler) Schedule(eval *Eval) (err error) {
	workers, err := ws.workers.ListWithCapacity(ws.pool, eval.MinCapacity)
	if err != nil {
		return errors.Wrap(err, "failed to list workers with enough capacity")
	}

	if len(workers) < 1 {
		return ErrPoolNotEnoughCapacity
	} else if len(workers) > 1 {
		sort.Slice(workers, func(i, j int) bool {
			return workers[i].Capacity >= workers[j].Capacity
		})
	}

	fmt.Println("AAAAA")

	worker := workers[0]
	if err = ws.workers.SubtractCapacity(worker.WorkerPK, eval.MinCapacity); err != nil {
		if err == ErrWorkerNotEnoughCapacity {
			return errors.Wrap(err, "worker capacity changed during scheduling")
		}

		return errors.Wrap(err, "failed to subtract worker capacity")
	}

	fmt.Println("BBBB")

	return nil
}
