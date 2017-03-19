//go:generate linegen
package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"

	"go.uber.org/zap"
)

//Task describes something that needs to run somewhere
type Task struct {
	Image string
}

//Alloc is a task assigned to a place where it will run
type Alloc struct {
	Task
	QueueURL string
}

//Run is an attempt at executing an allocation
type Run struct {
	Alloc
	Token string
}

func pullTaskRunFromActivity(activityARN string) (*Run, error) {
	return nil, nil
}

func sendTaskRunToQueue(run *Run) error {
	return nil
}

//Conf holds our configuration taken from the environment
type Conf struct {
	Deployment     string `envconfig:"DEPLOYMENT"`
	RunActivityARN string `envconfig:"RUN_ACTIVITY_ARN"`
}

// Context provides information about Lambda execution environment.
type Context struct {
	FunctionName          string       `json:"function_name"`
	FunctionVersion       string       `json:"function_version"`
	InvokedFunctionARN    string       `json:"invoked_function_arn"`
	MemoryLimitInMB       int          `json:"memory_limit_in_mb,string"`
	AWSRequestID          string       `json:"aws_request_id"`
	LogGroupName          string       `json:"log_group_name"`
	LogStreamName         string       `json:"log_stream_name"`
	RemainingTimeInMillis func() int64 `json:"-"`
}

//LambdaFunc describes a Lambda handler that matches a specific suffic
type LambdaFunc func(conf *Conf, logs *zap.Logger, ev interface{}) (interface{}, error)

//LambdaHandlers map arn suffixes to actual event handlers
var LambdaHandlers = map[*regexp.Regexp]LambdaFunc{
	regexp.MustCompile(`-alloc$`): func(conf *Conf, logs *zap.Logger, ev interface{}) (interface{}, error) {
		return "alloc", nil
	},

	regexp.MustCompile(`-dealloc$`): func(conf *Conf, logs *zap.Logger, ev interface{}) (interface{}, error) {
		return "dealloc", nil
	},

	//Dispatch pulls activitie tasks as executions and sends them to the correct worker queue. There is no way of only getting activity tasks that belongs to the same exeuction as the invocation of dispatch itself. As such, this handler should never fail and bring the (nested) statemachine itself in a failed state; only the activity can do that. @TODO add a backup cron event that allso dispatches activity tokens that may have been missed?
	regexp.MustCompile(`-dispatch$`): func(conf *Conf, logs *zap.Logger, ev interface{}) (interface{}, error) {

		//get an execution from the activity
		run, err := pullTaskRunFromActivity(conf.RunActivityARN)
		if err != nil {
			logs.Error("failed to pull task run from activity", zap.Error(err))
			return nil, nil
		}

		//send a run to the assigned queue
		err = sendTaskRunToQueue(run)
		if err != nil {
			logs.Error("failed to send run", zap.Error(err))
			return nil, nil
		}

		return nil, nil
	},
}

//Handle is the entrypoint to our Lambda core
func Handle(ev interface{}, ctx *Context) (interface{}, error) {
	logs, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v", err)
	}

	conf := &Conf{}
	err = envconfig.Process("LINE", conf)
	if err != nil {
		logs.Fatal("failed to process env config", zap.Error(err))
	}

	//report loaded configuration for debugging purposes
	logs.Info("loaded configuration", zap.String("conf", fmt.Sprintf("%+v", conf)))

	//find a handler that has a name that matches the the calling Lambda ARN
	var testedExp []string
	for exp, handler := range LambdaHandlers {
		if exp.MatchString(ctx.InvokedFunctionARN) {
			return handler(conf, logs.With(zap.String("λ", ctx.InvokedFunctionARN)), ev)
		}

		testedExp = append(testedExp, exp.String())
	}

	//error not found
	return nil, errors.Errorf("none of the tested handlers (%s) matched ARN '%s'", strings.Join(testedExp, ", "), ctx.InvokedFunctionARN)
}
