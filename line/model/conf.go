package model

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
)

//Conf configures the model
type Conf struct {
	Deployment         string `required:"true" envconfig:"DEPLOYMENT"`
	AWSRegion          string `required:"true" envconfig:"AWS_REGION"`
	AWSAccessKeyID     string `required:"true" envconfig:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string `required:"true" envconfig:"AWS_SECRET_ACCESS_KEY"`

	WorkersTableName string `envconfig:"TABLE_NAME_WORKERS"`
}

//ConfFromEnv will attempt to fill configuration from the process environment
func ConfFromEnv() (cfg *Conf, err error) {
	cfg = &Conf{}
	err = envconfig.Process("LINE", cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to process env config")
	}

	return cfg, nil
}
