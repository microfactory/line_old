package line

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
	"github.com/pressly/chi"
)

//FmtQueueURL is able to "predict" an sqs queue url from configurations
func FmtQueueURL(conf *Conf, poolID string) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s-%s", conf.AWSRegion, conf.AWSAccountID, conf.Deployment, poolID)
}

//FmtExecARN "predict" the execution arn as to not having to keep it in state arn:aws:states:eu-west-1:399106104436:execution:f9e4beb4-line001-advan-schedule:abc-532df0adac48da0bf5f9
func FmtExecARN(conf *Conf, projectID, taskID string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:execution:%s-schedule:%s-%s", conf.AWSRegion, conf.AWSAccountID, conf.Deployment, projectID, taskID)
}

//Mux sets up the HTTP multiplexer
func Mux(conf *Conf, svc *Services) http.Handler {
	r := chi.NewRouter()

	//
	// Create Task
	//
	r.Post("/:projectID/tasks", errh(func(w http.ResponseWriter, r *http.Request) (err error) {

		task := &Task{}
		dec := json.NewDecoder(r.Body)
		err = dec.Decode(task)
		if err != nil {
			return errors.Wrap(err, "failed to decode task")
		}

		if task.PoolID == "" {
			return errors.Errorf("no pool id provided")
		}

		idb := make([]byte, 10)
		_, err = rand.Read(idb)
		if err != nil {
			return errors.Wrap(err, "failed to read random id")
		}

		task.ProjectID = chi.URLParam(r, "projectID")
		task.TaskID = hex.EncodeToString(idb)
		if err := PutNewTask(conf, svc.DB, task); err != nil {
			return errors.Wrap(err, "failed to put task")
		}

		buf := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buf)
		err = enc.Encode(task)
		if err != nil {
			return errors.Wrap(err, "failed to encode task")
		}

		if _, err = svc.SFN.StartExecution(&sfn.StartExecutionInput{
			StateMachineArn: aws.String(conf.StateMachineARN),
			Input:           aws.String(buf.String()),
			Name: aws.String(
				fmt.Sprintf("%s-%s", task.ProjectID, task.TaskID),
			),
		}); err != nil {
			return errors.Wrap(err, "failed to schedule task")
		}

		_, err = io.Copy(w, buf)
		return err
	}))

	//
	// Delete Task
	//
	r.Delete("/:projectID/tasks/:taskID", errh(func(w http.ResponseWriter, r *http.Request) (err error) {
		tpk := TaskPK{
			TaskID:    chi.URLParam(r, "taskID"),
			ProjectID: chi.URLParam(r, "projectID"),
		}

		err = DeleteTask(conf, svc.DB, tpk)
		if err != nil {
			return errors.Wrap(err, "failed to remove task")
		}

		execARN := FmtExecARN(conf, tpk.ProjectID, tpk.TaskID)
		if _, err = svc.SFN.StopExecution(&sfn.StopExecutionInput{
			ExecutionArn: aws.String(execARN),
			Error:        aws.String("HumanAbort"),
			Cause:        aws.String("API client aborted execution"),
		}); err != nil {
			return errors.Wrap(err, "failed to stop execution")
		}

		return nil
	}))

	//
	// Create Pool
	//
	r.Post("/pools", errh(func(w http.ResponseWriter, r *http.Request) (err error) {

		idb := make([]byte, 10)
		_, err = rand.Read(idb)
		if err != nil {
			return errors.Wrap(err, "failed to read random id")
		}

		poolID := hex.EncodeToString(idb)
		var out *sqs.CreateQueueOutput
		if out, err = svc.SQS.CreateQueue(&sqs.CreateQueueInput{
			QueueName: aws.String(fmt.Sprintf("%s-%s", conf.Deployment, poolID)),
			Attributes: map[string]*string{
				"MessageRetentionPeriod": aws.String("60"),
			},
		}); err != nil {
			return errors.Wrap(err, "failed to create pool queue")
		}

		pool := &Pool{
			PoolPK:   PoolPK{PoolID: poolID},
			QueueURL: aws.StringValue(out.QueueUrl),
		}

		if err := PutNewPool(conf, svc.DB, pool); err != nil {
			return errors.Wrap(err, "failed to put pool")
		}

		w.WriteHeader(http.StatusCreated)
		enc := json.NewEncoder(w)
		return enc.Encode(pool)
	}))

	//
	// Delete Pool
	//
	r.Delete("/pools/:poolID", errh(func(w http.ResponseWriter, r *http.Request) (err error) {

		ppk := PoolPK{chi.URLParam(r, "poolID")}
		if err := DeletePool(conf, svc.DB, ppk); err != nil {
			return errors.Wrap(err, "failed to delete pool")
		}

		if _, err = svc.SQS.DeleteQueue(&sqs.DeleteQueueInput{
			QueueUrl: aws.String(FmtQueueURL(conf, ppk.PoolID)),
		}); err != nil {
			return errors.Wrap(err, "failed to delete pool queue")
		}

		return nil
	}))

	r.NotFound(notFoundHandler)
	r.MethodNotAllowed(methodNotAllowedHandler)
	return r
}

func errh(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			enc := json.NewEncoder(w)
			err = enc.Encode(struct {
				Message string
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
