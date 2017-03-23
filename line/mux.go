package line

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pressly/chi"
)

//Mux sets up the HTTP multiplexer
func Mux(conf *Conf, logs *zap.Logger, sess *session.Session) http.Handler {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, from chi")
	})

	return r
}
