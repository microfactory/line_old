#!/bin/bash
set -e

function print_help {
	printf "Available Commands:\n";
	awk -v sq="'" '/^function run_([a-zA-Z0-9-]*)\s*/ {print "-e " sq NR "p" sq " -e " sq NR-1 "p" sq }' make.sh \
		| while read line; do eval "sed -n $line make.sh"; done \
		| paste -d"|" - - \
		| sed -e 's/^/  /' -e 's/function run_//' -e 's/#//' -e 's/{/	/' \
		| awk -F '|' '{ print "  " $2 "\t" $1}' \
		| expand -t 30
}

function run_build { #compile handler
	docker run --rm                                                             \
		-e HANDLER=handler                                                      	\
		-e PACKAGE=handler                                                      	\
		-v $GOPATH:/go                                                           	\
		-v $(pwd):/tmp                                                          	\
		-w /go/src/github.com/microfactory/line                                   \
		eawsy/aws-lambda-go-shim:latest make all
	rm handler.so
}

function run_deploy { #deploys the infra
  export $(cat secrets.env)
  terraform apply \
		-var project=line \
		-var owner=$(git config user.name) \
		-var version=$(cat VERSION) \
			infra
}

function run_destroy { #destroy the infra
  export $(cat secrets.env)
  terraform destroy \
		-var project=line \
		-var owner=$(git config user.name) \
		-var version=$(cat VERSION) \
			infra
}

case $1 in
	"build") run_build ;;
	"deploy") run_deploy ;;
	"destroy") run_destroy ;;
	*) print_help ;;
esac
