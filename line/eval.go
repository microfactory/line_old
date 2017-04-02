package line

//Eval is a scheduling evaluation
type Eval struct {
	Size  int    `dynamodbav:"size"`
	Pool  string `dynamodbav:"pool"`
	Retry int    `dynamodbav:"try"`
}
