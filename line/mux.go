package line

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/microfactory/line/line/client"
	"github.com/pkg/errors"
	"github.com/pressly/chi"
)

//FmtReplicaID formats the combined pool and worker id of a replica
func FmtReplicaID(datasetID, workerID string) string {
	return fmt.Sprintf("%s:%s", datasetID, workerID)
}

//FmtWorkerQueueName will format a sqs queue name consistently
func FmtWorkerQueueName(conf *Conf, poolID, workerID string) string {
	return fmt.Sprintf("%s-%s-%s", conf.Deployment, poolID, workerID)
}

//FmtPoolQueueName will format a sqs queue name consistently
func FmtPoolQueueName(conf *Conf, poolID string) string {
	return fmt.Sprintf("%s-%s", conf.Deployment, poolID)
}

//FmtPoolQueueURL is able to "predict" an sqs queue url from configurations
func FmtPoolQueueURL(conf *Conf, poolID string) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", conf.AWSRegion, conf.AWSAccountID, FmtPoolQueueName(conf, poolID))
}

//FmtWorkerQueueURL is able to "predict" an sqs queue url from configurations
func FmtWorkerQueueURL(conf *Conf, poolID, workerID string) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", conf.AWSRegion, conf.AWSAccountID, FmtWorkerQueueName(conf, poolID, workerID))
}

//Mux sets up the HTTP multiplexer
func Mux(conf *Conf, svc *Services) http.Handler {
	r := chi.NewRouter()

	//
	// Create Pool
	//
	r.Post("/CreatePool", errh(func(w http.ResponseWriter, r *http.Request) (err error) {
		input := &client.CreatePoolInput{}
		err = decodeInput(r.Body, input)
		if err != nil {
			return err
		}

		idb := make([]byte, 10)
		_, err = rand.Read(idb)
		if err != nil {
			return errors.Wrap(err, "failed to generate random id bytes")
		}

		poolID := hex.EncodeToString(idb)
		var qout *sqs.CreateQueueOutput
		if qout, err = svc.SQS.CreateQueue(&sqs.CreateQueueInput{
			QueueName: aws.String(FmtPoolQueueName(conf, poolID)),
		}); err != nil {
			return errors.Wrap(err, "failed to create queue")
		}

		pool := &Pool{
			PoolPK:   PoolPK{poolID},
			QueueURL: aws.StringValue(qout.QueueUrl),
		}

		err = PutNewPool(conf, svc.DB, pool)
		if err != nil {
			return errors.Wrap(err, "failed to put new pool")
		}

		output := &client.CreatePoolOutput{
			PoolID: pool.PoolID,
		}

		return encodeOutput(w, output)
	}))

	//
	// Register Worker
	//
	r.Post("/RegisterWorker", errh(func(w http.ResponseWriter, r *http.Request) (err error) {
		input := &client.RegisterWorkerInput{}
		err = decodeInput(r.Body, input)
		if err != nil {
			return err
		}

		pool, err := GetActivePool(conf, svc.DB, PoolPK{input.PoolID})
		if err != nil {
			return errors.Wrap(err, "failed to get active pool")
		}

		idb := make([]byte, 10)
		_, err = rand.Read(idb)
		if err != nil {
			return errors.Wrap(err, "failed to generate random id bytes")
		}

		workerID := hex.EncodeToString(idb)
		var qout *sqs.CreateQueueOutput
		if qout, err = svc.SQS.CreateQueue(&sqs.CreateQueueInput{
			QueueName: aws.String(FmtWorkerQueueName(conf, pool.PoolID, workerID)),
		}); err != nil {
			return errors.Wrap(err, "failed to create queue")
		}

		worker := &Worker{
			WorkerPK: WorkerPK{
				WorkerID: workerID,
				PoolID:   pool.PoolID,
			},
			QueueURL: aws.StringValue(qout.QueueUrl),
			Capacity: input.Capacity,
			TTL:      time.Now().Unix() + conf.WorkerTTL,
		}

		err = PutNewWorker(conf, svc.DB, worker)
		if err != nil {
			return errors.Wrap(err, "failed to put worker")
		}

		output := &client.RegisterWorkerOutput{
			PoolID:   worker.PoolID,
			WorkerID: worker.WorkerID,
			QueueURL: worker.QueueURL,
			Capacity: worker.Capacity,
		}

		return encodeOutput(w, output)
	}))

	//
	// Disband Pool
	//
	r.Post("/DisbandPool", errh(func(w http.ResponseWriter, r *http.Request) (err error) {
		input := &client.DisbandPoolInput{}
		err = decodeInput(r.Body, input)
		if err != nil {
			return err
		}

		if _, err = svc.SQS.DeleteQueue(&sqs.DeleteQueueInput{
			QueueUrl: aws.String(FmtPoolQueueURL(conf, input.PoolID)),
		}); err != nil {
			return errors.Wrap(err, "failed to remove queue")
		}

		expire := time.Now().Unix() + conf.PoolTTL
		if err = UpdatePoolTTL(conf, svc.DB, expire, PoolPK{
			PoolID: input.PoolID,
		}); err != nil {
			return errors.Wrap(err, "failed to update pool ttl")
		}

		return encodeOutput(w, &client.DisbandPoolOutput{})
	}))

	//
	// SendHeartbeat
	//
	r.Post("/SendHeartbeat", errh(func(w http.ResponseWriter, r *http.Request) (err error) {
		input := &client.SendHeartbeatInput{}
		err = decodeInput(r.Body, input)
		if err != nil {
			return err
		}

		pool, err := GetActivePool(conf, svc.DB, PoolPK{input.PoolID})
		if err != nil {
			return errors.Wrap(err, "failed to get active pool")
		}

		now := time.Now().Unix()
		if err = UpdateWorkerTTL(conf, svc.DB, now+conf.WorkerTTL, WorkerPK{
			PoolID:   pool.PoolID,
			WorkerID: input.WorkerID,
		}); err != nil {
			return errors.Wrap(err, "failed to update worker ttl")
		}

		//update replicas, resetting the ttl
		for _, datasetID := range input.Datasets {
			replica := &Replica{
				ReplicaPK: ReplicaPK{
					PoolID:    pool.PoolID,
					ReplicaID: FmtReplicaID(datasetID, input.WorkerID),
				},
				TTL: now + conf.ReplicaTTL,
			}

			if err = PutReplica(conf, svc.DB, replica); err != nil {
				return errors.Wrapf(err, "failed to update replica: %+v", replica)
			}
		}

		//update allocs, moving the ttl futher into the future
		for _, allocID := range input.Allocs {
			apk := AllocPK{
				PoolID:  pool.PoolID,
				AllocID: allocID,
			}

			if err = UpdateAllocTTL(conf, svc.DB, now+conf.AllocTTL, apk); err != nil {
				return errors.Wrapf(err, "failed to update alloc ttl: %+v", allocID)
			}
		}

		return encodeOutput(w, &client.SendHeartbeatOutput{})
	}))

	//
	// ScheduleEval
	//
	r.Post("/ScheduleEval", errh(func(w http.ResponseWriter, r *http.Request) (err error) {
		input := &client.ScheduleEvalInput{}
		err = decodeInput(r.Body, input)
		if err != nil {
			return err
		}

		pool, err := GetActivePool(conf, svc.DB, PoolPK{input.PoolID})
		if err != nil {
			return errors.Wrap(err, "failed to get active pool")
		}

		msg, err := json.Marshal(&Eval{Size: input.Size, Dataset: input.DatasetID})
		if err != nil {
			return errors.Wrap(err, "failed to encode scheduling message")
		}

		if _, err := svc.SQS.SendMessage(&sqs.SendMessageInput{
			QueueUrl:    aws.String(pool.QueueURL),
			MessageBody: aws.String(string(msg)),
		}); err != nil {
			return errors.Wrap(err, "failed to send message")
		}

		return encodeOutput(w, &client.ScheduleEvalOutput{})
	}))

	//
	// CompleteAlloc
	//
	r.Post("/CompleteAlloc", errh(func(w http.ResponseWriter, r *http.Request) (err error) {
		input := &client.CompleteAllocInput{}
		err = decodeInput(r.Body, input)
		if err != nil {
			return err
		}

		pool, err := GetActivePool(conf, svc.DB, PoolPK{input.PoolID})
		if err != nil {
			return errors.Wrap(err, "failed to get active pool")
		}

		alloc, err := GetAlloc(conf, svc.DB, AllocPK{
			PoolID:  pool.PoolID,
			AllocID: input.AllocID,
		})
		if err != nil {
			return errors.Wrap(err, "failed to get alloc")
		}

		//@TODO send the output/exit code somewhere?
		//@TODO send releases on a queue(?)
		//@TODO resend to scheduling queue on failure
		//@TODO support for workflows, i.e auto-schedule new task?

		err = releaseAlloc(conf, svc, alloc)
		if err != nil {
			return errors.Wrap(err, "failed to release alloc")
		}

		return encodeOutput(w, &client.CompleteAllocOutput{})
	}))

	r.NotFound(notFoundHandler)
	r.MethodNotAllowed(methodNotAllowedHandler)
	return r
}

func decodeInput(r io.Reader, in interface{}) (err error) {
	dec := json.NewDecoder(r)
	err = dec.Decode(in)
	if err != nil {
		return errors.Wrap(err, "failed to decode input")
	}

	return nil
}

func encodeOutput(w io.Writer, out interface{}) (err error) {
	enc := json.NewEncoder(w)
	err = enc.Encode(out)
	if err != nil {
		return errors.Wrap(err, "failed to encode output")
	}
	return nil
}

func errh(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			enc := json.NewEncoder(w)
			err = enc.Encode(struct {
				Message string `json:"message"`
			}{err.Error()})
			if err != nil {
				fmt.Fprintln(w, `{"message": "failed to encode error"}`)
			}
		}
	}
}

func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	fmt.Fprintf(w, `{"message": "method not allowed"}`)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, `{"message": "page not found"}`)
}
