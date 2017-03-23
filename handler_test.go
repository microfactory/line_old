package main

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/kelseyhightower/envconfig"
	"github.com/microfactory/line/line"
	"github.com/nerdalize/nerd/nerd/payload"
	"go.uber.org/zap"
)

func TestNExecutions(t *testing.T) {
	logs, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v", err)
	}

	conf := &line.Conf{}
	err = envconfig.Process("LINE", conf)
	if err != nil {
		logs.Fatal("failed to process env config", zap.Error(err))
	}

	var sess *session.Session
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
		logs.Fatal("failed to setup aws session", zap.Error(err))
	}

	task := &payload.Task{
		TaskID:    fmt.Sprintf("t-%d", time.Now().UnixNano()),
		ProjectID: "sequonomics-1",
		Image:     "nginx",
	}
	msg, err := json.Marshal(task)
	if err != nil {
		logs.Fatal("failed to marshal task", zap.Error(err))
	}

	sfnconn := sfn.New(sess)
	for i := 0; i < 50; i++ {
		var exec *sfn.StartExecutionOutput
		if exec, err = sfnconn.StartExecution(&sfn.StartExecutionInput{
			StateMachineArn: aws.String(conf.StateMachineARN),
			Input:           aws.String(string(msg)),
		}); err != nil {
			logs.Fatal("failed to start execution", zap.Error(err))
		}

		logs.Info("started execution", zap.String("arn", aws.StringValue(exec.ExecutionArn)))
	}

}
