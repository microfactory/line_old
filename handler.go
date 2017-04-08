//go:generate linegen
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/microfactory/line/line"
	"github.com/microfactory/line/line/conf"
)

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

//Handle is the entrypoint to our Lambda core, it "routes" invocations from specific Lambda functions to specific handlers based  on a regexp of the function ARN
func Handle(ev json.RawMessage, ctx *Context) (interface{}, error) {
	logs, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v", err)
	}

	cfg := &conf.Conf{}
	err = envconfig.Process("LINE", cfg)
	if err != nil {
		logs.Fatal("failed to process env config", zap.Error(err))
	}

	sess, err := session.NewSession(
		&aws.Config{
			Region: aws.String(cfg.AWSRegion),
			Credentials: credentials.NewStaticCredentials(
				cfg.AWSAccessKeyID,
				cfg.AWSSecretAccessKey,
				"",
			),
		},
	)
	if err != nil {
		logs.Fatal("failed to setup aws session", zap.Error(err))
	}

	svc := &conf.Services{
		SQS:  sqs.New(sess),
		DB:   dynamodb.New(sess),
		Logs: logs,
	}

	//report loaded configuration for debugging purposes
	logs.Info("loaded configuration", zap.String("conf", fmt.Sprintf("%+v", cfg)), zap.String("ctx", fmt.Sprintf("%+v", ctx)))

	//find a handler that has a name that matches the the calling Lambda ARN
	var testedExp []string
	for exp, handler := range line.Handlers {
		if exp.MatchString(ctx.InvokedFunctionARN) {
			return handler(
				cfg,
				svc,
				ev,
			)
		}

		testedExp = append(testedExp, exp.String())
	}

	//error not found
	return nil, errors.Errorf("none of the tested handlers (%s) matched ARN '%s'", strings.Join(testedExp, ", "), ctx.InvokedFunctionARN)
}
