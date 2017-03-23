package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kelseyhightower/envconfig"
	"github.com/nerdalize/nerd/nerd/payload"
	"github.com/pkg/errors"
)

//TaskStatus describes the container status of a task
type TaskStatus struct {
	TaskContainer
	code   int      //exit code
	err    error    //application error
	mounts []string //mount ids
}

//TaskLogEvent is a log event for a certain task container
type TaskLogEvent struct {
	*TaskContainer
	t   time.Time
	msg string
}

//TaskContainer is a unique execution for a specific task
type TaskContainer struct {
	cid   string //container id
	tid   string //task id
	pid   string //project id
	token string //activity token
}

//Conf holds our configuration taken from the environment
type Conf struct {
	Deployment         string `envconfig:"DEPLOYMENT"`
	StateMachineARN    string `envconfig:"STATE_MACHINE_ARN"`
	RunActivityARN     string `envconfig:"RUN_ACTIVITY_ARN"`
	WorkersTableName   string `envconfig:"TABLE_WORKERS_NAME"`
	WorkersTableCapIdx string `envconfig:"TABLE_WORKERS_IDX_CAP"`
	AWSAccessKeyID     string `envconfig:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string `envconfig:"AWS_SECRET_ACCESS_KEY"`
	AWSRegion          string `envconfig:"AWS_REGION"`
	PoolQueueURL       string `envconfig:"POOL_QUEUE_URL"`
}

//Worker configures the process that pulls tasks if necessary
type Worker struct {
	LogGroupName string
	WorkerID     string
	QueueURL     string
}

func main() {
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	conf := &Conf{}
	err := envconfig.Process("LINE", conf)
	if err != nil {
		log.Fatal("failed to process env config", zap.Error(err))
	}

	worker := &Worker{
		LogGroupName: "foobar",
		QueueURL:     conf.PoolQueueURL,
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
		log.Fatal("failed to setup aws session", zap.Error(err))
	}

	messages := sqs.New(sess)
	states := sfn.New(sess)
	cwatch := cloudwatchlogs.New(sess)

	//for now, we just parse use the docker cli
	exe, err := exec.LookPath("docker")
	if err != nil {
		log.Fatal(err)
	}

	//this function in run concurrently for each task container and writes output from 'docker log' to an I/O pipe for scanning and event handling
	pipeLogs := func(w io.Writer, tc TaskContainer) {
		args := []string{"logs", "-f", "-t", tc.cid}
		cmd := exec.Command(exe, args...)
		cmd.Stdout = w
		cmd.Stderr = w
		err = cmd.Run() //blocks until command ends
		if err != nil {
			fmt.Println("err starting log pipe", err)
			//@TODO error following logs
		}

		//@TODO main routine signals will also cancel, so the cancelling routine could wait for us to send remaining stuff over.
	}

	//scanLogs will read a stream container output and split and parsed it into lines as log events that can be stored remotely.
	scanLogs := func(r io.Reader, evCh chan<- TaskLogEvent, tc TaskContainer) {
		logscan := bufio.NewScanner(r)
		for logscan.Scan() {
			fields := strings.SplitN(logscan.Text(), " ", 2)
			if len(fields) < 2 {
				fmt.Println("unexpected log line:", logscan.Text())
				//@TODO show error that log line was not of expected format
				continue
			}

			if fields[1] == "" {
				continue //ignore empty lines
			}

			ev := TaskLogEvent{TaskContainer: &tc, msg: fields[1]}
			if ev.t, err = time.Parse(time.RFC3339Nano, fields[0]); err != nil {
				fmt.Println("unexpected time stamp: ", err)
				//@TODO handle error better
				continue
			}

			evCh <- ev
		}
		if err := logscan.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to scan logs:", err)
		}
	}

	//pushLogs moves the actual log events to the platform it is responsible for batching events together as to not run into throttling issues or keeping state too long
	bufTimeout := time.Second * 5
	bufSize := 30
	pushLogs := func(evCh <-chan TaskLogEvent, group, stream string) {
		logsEvIn := &cloudwatchlogs.PutLogEventsInput{}
		put := func() {
			if logsEvIn.LogGroupName == nil {
				if _, err = cwatch.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
					LogGroupName:  aws.String(group),
					LogStreamName: aws.String(stream),
				}); err != nil {
					//@TODO better error handling
					// fmt.Println("failed to create stream:", err)
					return
				}

				logsEvIn.SetLogGroupName(group)
				logsEvIn.SetLogStreamName(stream)
			}

			var out *cloudwatchlogs.PutLogEventsOutput
			if out, err = cwatch.PutLogEvents(logsEvIn); err != nil {
				//@TODO what if put logs fails?
				fmt.Println("put logs failed:", err)
				return
			}

			logsEvIn.LogEvents = nil
			logsEvIn.SequenceToken = out.NextSequenceToken
		}

		for {
			to := time.After(bufTimeout)
			select {
			//@TODO send buffered logs when shutting down
			case <-to:
				if len(logsEvIn.LogEvents) > 0 {
					put()
				}
			case ev := <-evCh:
				logsEvIn.LogEvents = append(logsEvIn.LogEvents, &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(ev.t.UnixNano() / 1000 / 1000), //only milliseconds are accepted (visible)
					Message:   aws.String(fmt.Sprintf(`{"t": %d, "line": "%s"}`, ev.t.UnixNano(), ev.msg)),
				})

				if len(logsEvIn.LogEvents) >= bufSize {
					put()
				}
			}
		}
	}

	//the logging routine takes a containers found in the listing and fans out into a routine for each container that is resposible for shipping logs to the platform
	containerCh := make(chan TaskContainer)
	go func() {
		containers := map[string]io.Reader{}
		for taskc := range containerCh {
			//if the container wasn't seen before we setup a logging pipeline
			if _, ok := containers[taskc.cid]; !ok {
				groupName := worker.LogGroupName
				streamName := fmt.Sprintf("%s-%s-%s", taskc.tid, worker.WorkerID, taskc.cid)

				//@TODO create a remote log stream for first sequence token
				evCh := make(chan TaskLogEvent, bufSize)
				pr, pw := io.Pipe()
				containers[taskc.cid] = pr
				go pipeLogs(pw, taskc)
				go scanLogs(pr, evCh, taskc)
				go pushLogs(evCh, groupName, streamName)
			}
		}
	}()

	//receive tasks from the message queue and start the container run loop, it will attemp to create containers for tasks unconditionally if it keeps failing queue retry will backoff. If it succeeds, fails the feedback loop will notify
	cap := 5
	capCh := make(chan struct{}, cap)
	go func() {
		for range capCh {
			var out *sqs.ReceiveMessageOutput
			if out, err = messages.ReceiveMessage(&sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(worker.QueueURL),
				WaitTimeSeconds:     aws.Int64(0),
				MaxNumberOfMessages: aws.Int64(1),
			}); err != nil {
				fmt.Fprintf(os.Stderr, "failed to receive message: %+v", err)
				//@TODO report async errors
				return
			}

			if len(out.Messages) > 0 {
				for _, msg := range out.Messages {
					task := &payload.Task{}
					if err = json.Unmarshal([]byte(aws.StringValue(msg.Body)), task); err != nil {

						//@TODO throw deserialization errors
						fmt.Fprintf(os.Stderr, "failed to deserialize: %+v", err)
						return
					}

					//@TODO execute a pre-run heartbeat to prevent starting containers for delayed but outdated task tokens. if the heartbeat returns a timed out error don't attempt to start it: (dont forget to delete the message)

					fmt.Fprintf(os.Stderr, "starting task: %s, token: %x\n", task.TaskID, sha1.Sum([]byte(task.ActivityToken)))
					args := []string{
						"run", "-d",
						//@TODO add logging to aws
						fmt.Sprintf("--name=task-%x", sha1.Sum([]byte(task.ActivityToken))),
						fmt.Sprintf("--label=nerd-project=%s", task.ProjectID),
						fmt.Sprintf("--label=nerd-task=%s", task.TaskID),
						fmt.Sprintf("--label=nerd-token=%s", task.ActivityToken),
						fmt.Sprintf("-v=/in"), fmt.Sprintf("-v=/out"),
						fmt.Sprintf("-e=NERD_PROJECT_ID=%s", task.ProjectID),
						fmt.Sprintf("-e=NERD_TASK_ID=%s", task.TaskID),
					}

					if task.InputID != "" {
						args = append(args, fmt.Sprintf("-e=NERD_DATASET_INPUT=%s", task.InputID))
					}

					for key, val := range task.Environment {
						args = append(args, fmt.Sprintf("-e=%s=%s", key, val))
					}

					go func(msg *sqs.Message) {
						args = append(args, task.Image)
						cmd := exec.Command(exe, args...)
						cmd.Stderr = os.Stderr
						_ = cmd.Run() //any result is ok

						//delete message, state is persisted in Docker, it is no longer relevant
						if _, err := messages.DeleteMessage(&sqs.DeleteMessageInput{
							QueueUrl:      aws.String(worker.QueueURL),
							ReceiptHandle: msg.ReceiptHandle,
						}); err != nil {
							//@TODO error on return error
							fmt.Fprintf(os.Stderr, "failed to delete message: %+v", err)
							return
						}
					}(msg)
				}
			}
		}
	}()

	//the container loop feeds running task tokens to the feedback loop by polling the `docker ps` output
	pr, pw := io.Pipe()
	psTicker := time.NewTicker(time.Second * 5)
	go func() {
		for range psTicker.C {
			args := []string{"ps", "-a",
				"--no-trunc",
				"--filter=label=nerd-token",
				"--format={{.ID}}\t{{.Status}}\t{{.Label \"nerd-token\"}}\t{{.Label \"nerd-project\"}}\t{{.Label \"nerd-task\"}}\t{{.Mounts}}",
			}

			buf := bytes.NewBuffer(nil)

			cmd := exec.Command(exe, args...)
			cmd.Stdout = io.MultiWriter(pw, buf)
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				//@TODO handle errors
				fmt.Fprintln(os.Stderr, "failed to list containers: ", err)
				continue
			}

			n := bytes.Count(buf.Bytes(), []byte{'\n'})
			for i := (cap - n); i > 0; i-- {
				capCh <- struct{}{}
			}
		}
	}()

	//the scan loop will parse docker states into exit statuses
	scanner := bufio.NewScanner(pr)
	statusCh := make(chan TaskStatus)
	go func() {
		for scanner.Scan() {
			fields := strings.SplitN(scanner.Text(), "\t", 6)
			if len(fields) != 6 {
				// statusCh <- TaskStatus{TaskContainer{fields[0], fields[4], fields[3], fields[2]}, 255, errors.New("unexpected ps line")}
				continue //less then 2 fields, shouldnt happen, and unable to scope error the o project/task/token
			}

			//fields: | cid | status | token | project | task | mounts |
			taskc := TaskContainer{fields[0], fields[4], fields[3], fields[2]}

			//we would like to start routines that pipe log lines to cloudwatch

			//second field can be interpreted by reversing state .String() https://github.com/docker/docker/blob/b59ee9486fad5fa19f3d0af0eb6c5ce100eae0fc/container/state.go#L70
			status := fields[1]
			if strings.HasPrefix(status, "Up") || strings.HasPrefix(status, "Restarting") || status == "Removal In Progress" || status == "Created" {
				//container is not yet "done": still in progress without statuscode, send heartbeat and continue to next tick
				statusCh <- TaskStatus{taskc, -1, nil, strings.Split(fields[5], ",")}
				continue
			} else {
				//container has "exited" or is "dead"
				if status == "Dead" {
					//@See https://github.com/docker/docker/issues/5684
					// There is also a new(ish) container state called "dead", which is set when there were issues removing the container. This is of course a work around for this particular issue, which lets you go and investigate why there is the device or resource busy error (probably a race condition), in which case you can attempt to remove again, or attempt to manually fix (e.g. unmount any left-over mounts, and then remove).
					statusCh <- TaskStatus{taskc, 255, errors.New("failed to remove container"), strings.Split(fields[5], ",")}
					continue

				} else if strings.HasPrefix(status, "Exited") {
					right := strings.TrimPrefix(status, "Exited (")
					lefts := strings.SplitN(right, ")", 2)
					if len(lefts) != 2 {
						statusCh <- TaskStatus{taskc, 255, errors.New("unexpected exited format: " + status), strings.Split(fields[5], ",")}
						continue
					}

					//write actual status code, can be zero in case of success
					code, err := strconv.Atoi(lefts[0])
					if err != nil {
						statusCh <- TaskStatus{taskc, 255, errors.New("unexpected status code, not a number: " + status), strings.Split(fields[5], ",")}
						continue
					} else {
						statusCh <- TaskStatus{taskc, code, nil, strings.Split(fields[5], ",")}
						continue
					}

				} else {
					statusCh <- TaskStatus{taskc, 255, errors.New("unexpected status: " + status), strings.Split(fields[5], ",")}
					continue
				}
			}
		}
		if err := scanner.Err(); err != nil {
			//@TODO handle scanniong IO errors
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
	}()

	//the feedback loop holds a view of task states and tokens
	for {
		select {
		case <-sigCh: //exit our main loop
			return
		case statusEv := <-statusCh: //sync docker status

			//container is in a state we understand, pass on the container fanout logic
			containerCh <- statusEv.TaskContainer
			fmt.Fprintf(os.Stderr, "task-%x is %d\n", sha1.Sum([]byte(statusEv.token)), statusEv.code)

			var err error
			if statusEv.code < 0 {
				fmt.Fprintln(os.Stderr, "heartbeat!")
				_, err = states.SendTaskHeartbeat(&sfn.SendTaskHeartbeatInput{
					TaskToken: aws.String(statusEv.token),
				})
			} else if statusEv.code == 0 {

				//we're gonna read the output id from a file inside the container using the `docker cp` command and trusting that when successfull an output dataset id was written in the correct location.
				idbuf := bytes.NewBuffer(nil)
				var outdata []byte
				func() { //new scope to prevent err shadowing

					tarbuf := bytes.NewBuffer(nil)
					outcmd := exec.Command(exe, "cp", fmt.Sprintf("%s:/out/.dataset", statusEv.cid), "-")
					outcmd.Stdout = tarbuf
					err := outcmd.Run()
					if err != nil {
						fmt.Println("failed to run dataset cp", err)
					}

					tr := tar.NewReader(tarbuf)
					for {
						_, err := tr.Next()
						if err == io.EOF {
							break // end of tar archive
						}

						if _, err := io.Copy(idbuf, tr); err != nil {
							log.Println("failed to read tar file")
							continue
						}
					}

					if idbuf.Len() == 0 {
						fmt.Fprint(idbuf, "d-ffffffff")
					}

					if outdata, err = json.Marshal(&payload.TaskResult{
						ProjectID:  statusEv.pid,
						TaskID:     statusEv.tid,
						OutputID:   strings.TrimSpace(idbuf.String()),
						ExitStatus: fmt.Sprintf("Exit Status: %d", statusEv.code),
					}); err != nil {
						fmt.Println("failed to marshal task result: ", err)
						return
					}
				}()

				//success
				fmt.Fprintln(os.Stderr, "success!")
				_, err = states.SendTaskSuccess(&sfn.SendTaskSuccessInput{
					TaskToken: aws.String(statusEv.token),
					Output:    aws.String(string(outdata)),
				})

			} else {
				//failure
				fmt.Fprintln(os.Stderr, "failed!")
				//@TODO dont send cause if .err is nil
				_, err = states.SendTaskFailure(&sfn.SendTaskFailureInput{
					TaskToken: aws.String(statusEv.token),
					Error:     aws.String(fmt.Sprintf(`{"error": "%d"}`, statusEv.code)),
					Cause:     aws.String(fmt.Sprintf(`{"cause": "%v"}`, statusEv.err)),
				})
			}

			if err != nil {
				aerr, ok := err.(awserr.Error)
				if !ok {
					fmt.Println("unexpected non-aws error:", err)
					//@TODO not an aws error, connection issues or otherwise, do not undertake an specific action maybe next time it will be better
					continue
				}

				if aerr.Code() == sfn.ErrCodeTaskTimedOut {
					fmt.Println("aws err:", aerr)
					cmd := exec.Command(exe, "stop", statusEv.cid)
					err = cmd.Run()
					if err != nil {
						fmt.Println("failed to stop task container:", statusEv.cid, statusEv.code, err)
						//@TODO report error
					}

					cmd = exec.Command(exe, "rm", statusEv.cid)
					err = cmd.Run()
					if err != nil {
						fmt.Println("failed to remove timed out task container:", statusEv.cid, statusEv.code, err)
						//@TODO report error
					}
				}
			}
		}
	}

}
