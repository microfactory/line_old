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

function run_work { #test our worker logic
	echo "--> working..."
	export $(terraform output env | tr -d ' '); go run ./work/main.go
}

function run_test { #test our scheduling reactor
	echo "--> testing..."
	export $(terraform output env | tr -d ' '); go test -v
}

function run_install { #install go dependencies
	command -v glide >/dev/null 2>&1 || { echo "executable 'glide' (dependency manager) must be installed: https://github.com/Masterminds/glide" >&2; exit 1; }

	echo "--> installing..."
	glide install
}

function run_build { #compile lambda handler
	command -v docker >/dev/null 2>&1 || { echo "executable 'docker' (container runtime client) must be installed: https://www.docker.com/" >&2; exit 1; }

	echo "--> building..."
	docker run --rm                                                             \
		-e HANDLER=handler                                                      	\
		-e PACKAGE=handler                                                      	\
		-v $GOPATH:/go                                                           	\
		-v $(pwd):/tmp                                                          	\
		-w /go/src/github.com/microfactory/line                                   \
		eawsy/aws-lambda-go-shim:latest bash -c "go build -v -buildmode=plugin -ldflags='-w -s' -o handler.so; pack handler handler.so handler.zip"
}

function run_deploy { #deploy infrastructure resources
	command -v terraform >/dev/null 2>&1 || { echo "executable 'terraform' (infrastructure manager) must be installed: https://www.terraform.io/" >&2; exit 1; }

	export $(cat secrets.env)
	echo "--> deploying..."
  terraform apply \
		-var project=line \
		-var owner=$(git config user.name) \
		-var version=$(cat VERSION) \
			infra
}

function run_destroy { #destroy deployed infrastructure
	command -v terraform >/dev/null 2>&1 || { echo "infrastructure manager executable 'terraform' must be installed: https://www.terraform.io/" >&2; exit 1; }

  export $(cat secrets.env)
	echo "--> destroying..."
  terraform destroy \
		-var project=line \
		-var owner=$(git config user.name) \
		-var version=$(cat VERSION) \
			infra
}

case $1 in
	"work") run_work ;;
	"test") run_test ;;
	"install") run_install ;;
	"build") run_build ;;
	"deploy") run_deploy ;;
	"destroy") run_destroy ;;
	*) print_help ;;
esac
