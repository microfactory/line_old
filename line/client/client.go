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
	case *DisbandPoolInput:
		loc.Path = path.Join(loc.Path, "DisbandPool")
	case *RegisterWorkerInput:
		loc.Path = path.Join(loc.Path, "RegisterWorker")
	case *SendHeartbeatInput:
		loc.Path = path.Join(loc.Path, "SendHeartbeat")
	case *ScheduleEvalInput:
		loc.Path = path.Join(loc.Path, "ScheduleEval")
	case *CompleteAllocInput:
		loc.Path = path.Join(loc.Path, "CompleteAlloc")
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

//RegisterWorker will setup a worker that provides capacity to a pool
func (c *Client) RegisterWorker(in *RegisterWorkerInput) (out *RegisterWorkerOutput, err error) {
	out = &RegisterWorkerOutput{}
	err = c.doRequest(in, out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do HTTP request")
	}

	return out, nil
}

//DisbandPool will remove a pool
func (c *Client) DisbandPool(in *DisbandPoolInput) (out *DisbandPoolOutput, err error) {
	out = &DisbandPoolOutput{}
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

//CompleteAlloc indicates to the server that an allocation has ended
func (c *Client) CompleteAlloc(in *CompleteAllocInput) (out *CompleteAllocOutput, err error) {
	out = &CompleteAllocOutput{}
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
