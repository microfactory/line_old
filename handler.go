//go:generate linegen
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//WorkerPK uniquely identifies a worker
type WorkerPK struct {
	PoolID   string `dynamodbav:"pool"`
	QueueURL string `dynamodbav:"que"`
}

//Worker represents a source of capacity
type Worker struct {
	WorkerPK
	Capacity int `dynamodbav:"cap"`
}

//Task describes something that needs to run somewhere
type Task struct {
	Size int
}

//Alloc is a task assigned to a place where it will run
type Alloc struct {
	*Task
	WorkerPK
	Result map[string]interface{}
}

//Conf holds our configuration taken from the environment
type Conf struct {
	Deployment         string `envconfig:"DEPLOYMENT"`
	RunActivityARN     string `envconfig:"RUN_ACTIVITY_ARN"`
	WorkersTableName   string `envconfig:"TABLE_WORKERS_NAME"`
	WorkersTableCapIdx string `envconfig:"TABLE_WORKERS_IDX_CAP"`
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
type LambdaFunc func(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (interface{}, error)

//LambdaHandlers map arn suffixes to actual event handlers
var LambdaHandlers = map[*regexp.Regexp]LambdaFunc{

	//Allocate will query workers with enough capacity to handle the incoming tasks. It will query for workers with enough capacity and update the table with a conditional to avoid races for the same capacity
	regexp.MustCompile(`-alloc$`): func(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (res interface{}, err error) {
		dbconn := dynamodb.New(sess)

		task := &Task{Size: 1}

		//query our secondary index for workers with capacity of at least the size of the task we require. @TODO apply conditional filters on results for additional selection
		taskSize, err := dynamodbattribute.Marshal(task.Size)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal task size")
		}

		//query workers with enough capacity at this point-in-time
		var qout *dynamodb.QueryOutput
		if qout, err = dbconn.Query(&dynamodb.QueryInput{
			TableName: aws.String(conf.WorkersTableName),
			IndexName: aws.String(conf.WorkersTableCapIdx),
			Limit:     aws.Int64(10),
			KeyConditionExpression: aws.String("#pool = :poolID AND #cap >= :taskSize"),
			ExpressionAttributeNames: map[string]*string{
				"#pool": aws.String("pool"),
				"#cap":  aws.String("cap"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":poolID":   {S: aws.String("p1")},
				":taskSize": taskSize,
			},
		}); err != nil {
			return nil, errors.Wrap(err, "failed to query workers")
		}

		//decode dynamo items into candidate workers
		var candidates []*Worker
		for _, item := range qout.Items {
			cand := &Worker{}
			err := dynamodbattribute.UnmarshalMap(item, cand)
			if err != nil {
				logs.Error("failed to unmarshal item", zap.Error(err))
				continue
			}

			candidates = append(candidates, cand)
		}

		//sort by workers with the highest capacity (spread) @TODO add a way of placing on lowest capacity, this will create more contention but allows more room for large placements in the future
		logs.Info("received candidate workers", zap.Int("candidates", len(candidates)))
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Capacity >= candidates[j].Capacity
		})

		//if we have no candidates to begin we return an error en hope it will be better in the future
		if len(candidates) < 1 {
			return nil, errors.Errorf("not enough capacity")
		}

		//then continue updating the selected worker's capacity to claim it, after this the capacity is allocated
		worker := candidates[0]
		pk, err := dynamodbattribute.MarshalMap(worker.WorkerPK)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal worker pk")
		}

		logs.Info("claim capacity of worker", zap.String("pool", worker.PoolID), zap.String("queue", worker.QueueURL))
		if _, err = dbconn.UpdateItem(&dynamodb.UpdateItemInput{
			TableName:           aws.String(conf.WorkersTableName),
			Key:                 pk,
			UpdateExpression:    aws.String(`SET cap = cap - :claim`),
			ConditionExpression: aws.String("cap >= :claim"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":claim": taskSize,
			},
		}); err != nil {
			return nil, errors.Wrap(err, "failed to update worker capacity")
		}

		return &Alloc{
			Task:     task,
			WorkerPK: worker.WorkerPK,
		}, nil
	},

	//Dealloc is responsible from freeing any capacity claimed by the task that just finished (successfully or not). If the task failed an application error is returned that will cause the outer branch to retry.
	regexp.MustCompile(`-dealloc$`): func(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (res interface{}, err error) {
		alloc := &Alloc{}
		err = json.Unmarshal(ev, alloc)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal input")
		}

		taskSize, err := dynamodbattribute.Marshal(alloc.Task.Size)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal task size")
		}

		pk, err := dynamodbattribute.MarshalMap(alloc.WorkerPK)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal worker pk")
		}

		dbconn := dynamodb.New(sess)
		if _, err := dbconn.UpdateItem(&dynamodb.UpdateItemInput{
			TableName:        aws.String(conf.WorkersTableName),
			Key:              pk,
			UpdateExpression: aws.String(`SET cap = cap + :taskSize`),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":taskSize": taskSize,
			},
		}); err != nil {
			return nil, errors.Wrap(err, "failed to release capacity back to worker")
		}

		//this is were we actually allow the allocation as a whole to fail such it can be retried on a different schedule
		if allocErr, hasError := alloc.Result["Error"]; hasError {
			return nil, fmt.Errorf("allocation failed: %+v", allocErr)
		}

		return ev, nil
	},

	//Dispatch pulls activitie tasks as executions and sends them to the correct worker queue. There is no way of only getting activity tasks that belongs to the same exeuction as the invocation of dispatch itself. As such, this handler should never fail and bring the (nested) statemachine itself in a failed state; only the activity can do that. @TODO add a backup cron event that allso dispatches activity tokens that may have been missed due to dispatch misalignment?
	regexp.MustCompile(`-dispatch$`): func(conf *Conf, logs *zap.Logger, sess *session.Session, ev json.RawMessage) (res interface{}, err error) {

		//get the run activity task, @TODO randomize the rate at which we get activity task input to force the misalignment between executions
		sfnconn := sfn.New(sess)
		var out *sfn.GetActivityTaskOutput
		if out, err = sfnconn.GetActivityTask(&sfn.GetActivityTaskInput{
			ActivityArn: aws.String(conf.RunActivityARN),
		}); err != nil {
			logs.Error("failed to get activity task", zap.Error(err))
			return ev, nil
		}

		//@TODO make sure to send the task run that ships with the activity, not the input one
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		n := r.Intn(10)
		logs.Info("dice rolled", zap.Int("nr", n))
		if n < 5 { //has a smaller chance to fail, should eventually converge all tasks succeeding
			if _, err = sfnconn.SendTaskFailure(&sfn.SendTaskFailureInput{
				Error:     aws.String("MyError"),
				Cause:     aws.String("some client side error"),
				TaskToken: out.TaskToken,
			}); err != nil {
				logs.Error("failed to send task failure", zap.Error(err))
				return ev, nil
			}
		} else {
			if _, err = sfnconn.SendTaskSuccess(&sfn.SendTaskSuccessInput{
				Output:    aws.String(`{"out": "put"}`),
				TaskToken: out.TaskToken,
			}); err != nil {
				logs.Error("failed to send task success", zap.Error(err))
				return ev, nil
			}
		}

		return ev, nil
	},
}

//Handle is the entrypoint to our Lambda core
func Handle(ev json.RawMessage, ctx *Context) (interface{}, error) {
	logs, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v", err)
	}

	conf := &Conf{}
	err = envconfig.Process("LINE", conf)
	if err != nil {
		logs.Fatal("failed to process env config", zap.Error(err))
	}

	sess, err := session.NewSession()
	if err != nil {
		logs.Fatal("failed to setup aws session", zap.Error(err))
	}

	//report loaded configuration for debugging purposes
	logs.Info("loaded configuration", zap.String("conf", fmt.Sprintf("%+v", conf)), zap.String("ctx", fmt.Sprintf("%+v", ctx)))

	//find a handler that has a name that matches the the calling Lambda ARN
	var testedExp []string
	for exp, handler := range LambdaHandlers {
		if exp.MatchString(ctx.InvokedFunctionARN) {
			return handler(
				conf,
				logs.With(zap.String("λ", ctx.InvokedFunctionARN)),
				sess,
				ev,
			)
		}

		testedExp = append(testedExp, exp.String())
	}

	//error not found
	return nil, errors.Errorf("none of the tested handlers (%s) matched ARN '%s'", strings.Join(testedExp, ", "), ctx.InvokedFunctionARN)
}
