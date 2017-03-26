package main

import (
	"log"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/microfactory/line/line"
	"go.uber.org/zap"
)

func TestNExecutions(t *testing.T) {
	logs, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v", err)
	}

	conf := &line.Conf{}
	err = envconfig.Process("LINE", conf)
	if err != nil {
		logs.Fatal("failed to process env config", zap.Error(err))
	}

	_ = conf

}
