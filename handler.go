package main

import "github.com/eawsy/aws-lambda-go-core/service/lambda/runtime"

//Handle is the main entrypoint to the engine
func Handle(evt interface{}, ctx *runtime.Context) (string, error) {
	return "Hello, World!", nil
}
