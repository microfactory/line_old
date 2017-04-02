package line

//Eval is a scheduling evaluation
type Eval struct {
	Dataset string `dynamodbav:"set"`  //certain dataset must be available
	Size    int    `dynamodbav:"size"` //certain capacity must be available
	Pool    string `dynamodbav:"pool"` //capacity must be in this pool
	Retry   int    `dynamodbav:"try"`
}
