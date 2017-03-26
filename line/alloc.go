package line

import (
	"encoding/json"
)

//HandleAlloc is a Lambda handler that periodically reads from the scheduling queue and queries the workers table for available capacity. If the capacity can be claimed an allocation is created.
func HandleAlloc(conf *Conf, svc *Services, ev json.RawMessage) (res interface{}, err error) {
	return ev, nil
}
