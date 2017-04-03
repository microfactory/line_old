package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
)

//Client facilitates communication with the line server
type Client struct {
	ep   *url.URL
	http *http.Client
	aws  *session.Session
}

//NewClient sets up an HTTP client that communicates with the server
func NewClient(endpoint string, aws *session.Session) (c *Client, err error) {
	c = &Client{
		http: http.DefaultClient,
		aws:  aws,
	}
	c.ep, err = url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse provided API endpoint")
	}

	return c, nil
}

func (c *Client) doRequest(in interface{}, out interface{}) (err error) {
	loc := *c.ep
	switch in.(type) {
	case *CreatePoolInput:
		loc.Path = path.Join(loc.Path, "CreatePool")
	case *DeletePoolInput:
		loc.Path = path.Join(loc.Path, "DeletePool")
	case *CreateWorkerInput:
		loc.Path = path.Join(loc.Path, "CreateWorker")
	case *DeleteWorkerInput:
		loc.Path = path.Join(loc.Path, "DeleteWorker")
	case *SendHeartbeatInput:
		loc.Path = path.Join(loc.Path, "SendHeartbeat")
	case *ScheduleEvalInput:
		loc.Path = path.Join(loc.Path, "ScheduleEval")
	default:
		return errors.Errorf("no known endpoint for %T", in)
	}

	reqBody := bytes.NewBuffer(nil)
	enc := json.NewEncoder(reqBody)
	err = enc.Encode(in)
	if err != nil {
		return errors.Wrap(err, "failed to wrap request input")
	}

	req, err := http.NewRequest("POST", loc.String(), reqBody)
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to execute HTTP request")
	}

	defer resp.Body.Close()
	respBody := bytes.NewBuffer(nil)
	tr := io.TeeReader(resp.Body, respBody)
	dec := json.NewDecoder(tr)
	err = dec.Decode(out)
	if err != nil {
		return errors.Wrapf(err, "unable to decode response body: '%s'", respBody.String())
	}

	if resp.StatusCode > 399 {
		return errors.Errorf("unexpected response code '%d' from server, url: '%s' response: '%s'", resp.StatusCode, loc.String(), respBody.String())
	}

	return nil
}

//CreatePool sets up a new capacity pool
func (c *Client) CreatePool(in *CreatePoolInput) (out *CreatePoolOutput, err error) {
	out = &CreatePoolOutput{}
	err = c.doRequest(in, out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do HTTP request")
	}

	return out, nil
}

//CreateWorker will setup a worker that provides capacity to a pool
func (c *Client) CreateWorker(in *CreateWorkerInput) (out *CreateWorkerOutput, err error) {
	out = &CreateWorkerOutput{}
	err = c.doRequest(in, out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do HTTP request")
	}

	return out, nil
}

//DeleteWorker will remove a worker from the pool
func (c *Client) DeleteWorker(in *DeleteWorkerInput) (out *DeleteWorkerOutput, err error) {
	out = &DeleteWorkerOutput{}
	err = c.doRequest(in, out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do HTTP request")
	}

	return out, nil
}

//DeletePool will remove a pool
func (c *Client) DeletePool(in *DeletePoolInput) (out *DeletePoolOutput, err error) {
	out = &DeletePoolOutput{}
	err = c.doRequest(in, out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do HTTP request")
	}

	return out, nil
}

//SendHeartbeat will submit a periodic report that updates ttls of various entities under the workers responsibility
func (c *Client) SendHeartbeat(in *SendHeartbeatInput) (out *SendHeartbeatOutput, err error) {
	out = &SendHeartbeatOutput{}
	err = c.doRequest(in, out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do HTTP request")
	}

	return out, nil
}

//ScheduleEval will queue up an evaluation to be processed by the scheduling logic
func (c *Client) ScheduleEval(in *ScheduleEvalInput) (out *ScheduleEvalOutput, err error) {
	out = &ScheduleEvalOutput{}
	err = c.doRequest(in, out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do HTTP request")
	}
	return out, nil
}

//ReceiveAllocs will open a long poll for new allocations
func (c *Client) ReceiveAllocs(in *ReceiveAllocsInput) (out *ReceiveAllocsOutput, err error) {
	recv, err := sqs.New(c.aws).ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(in.WorkerQueueURL),
		MaxNumberOfMessages: aws.Int64(in.MaxNumberOfMessages),
		WaitTimeSeconds:     aws.Int64(in.WaitTimeSeconds),
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to receive allocs")
	}

	out = &ReceiveAllocsOutput{}
	for _, msg := range recv.Messages {
		alloc := &Alloc{}
		err := json.Unmarshal([]byte(aws.StringValue(msg.Body)), alloc)
		if err != nil {
			return nil, errors.Wrap(err, "unable to decode alloc message")
		}

		out.Allocs = append(out.Allocs, alloc)
	}

	return out, nil
}
