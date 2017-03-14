package main

import (
	"fmt"
	"log"

	"go.uber.org/zap"
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

//Handle is the entrypoint to our Lambda core
func Handle(evt interface{}, ctx *Context) (interface{}, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v", err)
	}

	//@TODO route to correct sub handler, based on function ARN
	//@TODO adjust event parsing based on expected source
	logger.Info("hello, world!", zap.String("λ", fmt.Sprintf("%+v", ctx.InvokedFunctionARN)))

	return ctx, nil
}
