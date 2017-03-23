package line

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/nerdalize/nerd/nerd/payload"
	"go.uber.org/zap"
)

//HandleDispatch is a Lambda handler that periodically reads activity tasks of the state machine executions and sens them to the correct pool queue
func HandleDispatch(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (res interface{}, err error) {
	sfnconn := sfn.New(sess)
	sqsconn := sqs.New(sess)

	for {
		var out *sfn.GetActivityTaskOutput
		if out, err = sfnconn.GetActivityTask(&sfn.GetActivityTaskInput{
			ActivityArn: aws.String(conf.RunActivityARN),
		}); err != nil {
			logs.Error("failed to get activity task", zap.Error(err))
			continue
		}

		if out.TaskToken == nil || aws.StringValue(out.TaskToken) == "" {
			logs.Info("no task token, breaking")
			break
		}

		task := &payload.Task{}
		err = json.Unmarshal([]byte(aws.StringValue(out.Input)), task)
		if err != nil {
			logs.Error("failed to unmarshal task", zap.Error(err))
			continue
		}

		task.ActivityToken = aws.StringValue(out.TaskToken)
		go func() {
			msg, err := json.Marshal(task)
			if err != nil {
				logs.Error("failed to marshal task", zap.Error(err))
				return
			}

			if _, err = sqsconn.SendMessage(&sqs.SendMessageInput{
				QueueUrl:    aws.String(conf.PoolQueueURL),
				MessageBody: aws.String(string(msg)),
			}); err != nil {
				logs.Error("failed to send message", zap.Error(err))
				return
			}

			logs.Info("send task token to pool")
		}()
	}

	return ev, nil
}
