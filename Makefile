# Image URL to use all building/pushing image targets
IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	# Exclude cmd/ as it has no tests and causes issues with -coverprofile
	go test $$(go list ./... | grep -v /cmd/) -coverprofile coverage.out

.PHONY: lint
lint: ## Run golangci-lint against code.
	golangci-lint run

##@ Build

.PHONY: build
build: fmt vet ## Build controller binary.
	go build -o bin/controller cmd/controller/main.go

.PHONY: run
run: fmt vet ## Run controller from your host.
	go run cmd/controller/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the controller.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the controller.
	docker push ${IMG}

##@ Deployment

.PHONY: generate-certs
generate-certs: ## Generate self-signed certificates for local testing.
	./hack/generate_certs.sh

.PHONY: deploy
deploy: ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	kubectl apply -k config/

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	kubectl delete -k config/

.PHONY: deploy-sample
deploy-sample: ## Deploy sample application with warmup enabled.
	kubectl apply -f config/samples/sample_deployment.yaml

.PHONY: undeploy-sample
undeploy-sample: ## Remove sample application.
	kubectl delete -f config/samples/sample_deployment.yaml

##@ Clean

.PHONY: clean
clean: ## Clean build artifacts.
	rm -rf bin/ dist/ coverage.out coverage.html
