package line

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pressly/chi"
)

//FmtQueueURL is able to "predict" an sqs queue url from configurations
func FmtQueueURL(conf *Conf, poolID string) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s-%s", conf.AWSRegion, conf.AWSAccountID, conf.Deployment, poolID)
}

//Mux sets up the HTTP multiplexer
func Mux(conf *Conf, svc *Services) http.Handler {
	r := chi.NewRouter()

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
