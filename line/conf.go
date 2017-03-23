package line

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
