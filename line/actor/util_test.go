package actor_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kelseyhightower/envconfig"
	"github.com/microfactory/line/line/conf"
	"go.uber.org/zap"
)

func testconf(tb testing.TB) (cfg *conf.Conf) {
	cfg = &conf.Conf{}
	err := envconfig.Process("LINE", cfg)
	if err != nil {
		tb.Fatal("failed to process env config", zap.Error(err))
	}

	return cfg
}

func awssess(tb testing.TB, conf *conf.Conf) (sess *session.Session) {
	var err error
	if sess, err = session.NewSession(
		&aws.Config{
			Region: aws.String(conf.AWSRegion),
			Credentials: credentials.NewStaticCredentials(
				conf.AWSAccessKeyID,
				conf.AWSSecretAccessKey,
				"",
			),
		},
	); err != nil {
		tb.Fatal("failed to setup aws session", zap.Error(err))
	}
	return sess
}

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}
