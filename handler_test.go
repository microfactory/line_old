package main

import (
	"log"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

func Test10Executions(t *testing.T) {
	logs, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v", err)
	}

	conf := &Conf{}
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

	sfnconn := sfn.New(sess)
	for i := 0; i < 20; i++ {
		var exec *sfn.StartExecutionOutput
		if exec, err = sfnconn.StartExecution(&sfn.StartExecutionInput{
			StateMachineArn: aws.String(conf.StateMachineARN),
			Input:           aws.String(`{"size": 3}`),
		}); err != nil {
			logs.Fatal("failed to start execution", zap.Error(err))
		}

		logs.Info("started execution", zap.String("arn", aws.StringValue(exec.ExecutionArn)))
	}

}
