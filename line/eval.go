package line

//Eval is a scheduling evaluation
type Eval struct {
	Dataset string `dynamodbav:"set"`  //certain dataset must be available
	Size    int    `dynamodbav:"size"` //certain capacity must be available
	Retry   int    `dynamodbav:"try"`
}
