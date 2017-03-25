package line

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/zap"
)

//HandleDispatch is a Lambda handler that periodically reads activity tasks of the state machine executions and sends them to the correct pool queue
func HandleDispatch(conf *Conf, svc *Services, ev json.RawMessage) (res interface{}, err error) {

	for {
		var out *sfn.GetActivityTaskOutput
		if out, err = svc.SFN.GetActivityTask(&sfn.GetActivityTaskInput{
			ActivityArn: aws.String(conf.RunActivityARN),
		}); err != nil {
			svc.Logs.Error("failed to get activity task", zap.Error(err))
			continue
		}

		if out.TaskToken == nil || aws.StringValue(out.TaskToken) == "" {
			svc.Logs.Info("no task token, breaking")
			break
		}

		task := &Task{}
		err = json.Unmarshal([]byte(aws.StringValue(out.Input)), task)
		if err != nil {
			svc.Logs.Error("failed to unmarshal task", zap.Error(err))
			continue
		}

		if task.PoolID == "" {
			svc.Logs.Error("received task without pool id", zap.String("task", task.TaskID))
		}

		task.ActivityToken = aws.StringValue(out.TaskToken)
		go func() {
			msg, err := json.Marshal(task)
			if err != nil {
				svc.Logs.Error("failed to marshal task", zap.Error(err))
				return
			}

			queueURL := FmtQueueURL(conf, task.PoolID)
			if _, err = svc.SQS.SendMessage(&sqs.SendMessageInput{
				QueueUrl:    aws.String(queueURL),
				MessageBody: aws.String(string(msg)),
			}); err != nil {
				svc.Logs.Error("failed to send message", zap.Error(err))
				return
			}

			svc.Logs.Info("send task token to pool")
		}()
	}

	return ev, nil
}
