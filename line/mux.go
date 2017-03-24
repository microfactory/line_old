package line

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
	"github.com/pressly/chi"
)

//FmtQueueURL is able to "predict" an sqs queue url from configurations
func FmtQueueURL(conf *Conf, poolID string) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s-%s", conf.AWSRegion, conf.AWSAccountID, conf.Deployment, poolID)
}

//Mux sets up the HTTP multiplexer
func Mux(conf *Conf, svc *Services) http.Handler {
	r := chi.NewRouter()

	//
	// Create Pool
	//
	r.Post("/pool", errh(func(w http.ResponseWriter, r *http.Request) (err error) {

		idb := make([]byte, 10)
		_, err = rand.Read(idb)
		if err != nil {
			return errors.Wrap(err, "failed to read random id")
		}

		poolID := hex.EncodeToString(idb)
		var out *sqs.CreateQueueOutput
		if out, err = svc.SQS.CreateQueue(&sqs.CreateQueueInput{
			QueueName: aws.String(fmt.Sprintf("%s-%s", conf.Deployment, poolID)),
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
	r.Delete("/pool/:poolID", errh(func(w http.ResponseWriter, r *http.Request) (err error) {

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
