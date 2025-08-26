.PHONY: all build test testacc install docs fmt lint tools clean

VERSION=0.1.0
ORGANIZATION=geldata
ARCH?=$$(uname -m | sed 's/x86_64/amd64/g')
KERNEL?=$$(uname -s | tr '[:upper:]' '[:lower:]')
HOSTNAME=registry.terraform.io
NAMESPACE=geldata
NAME=daytona

all: build

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/terraform-provider-daytona

test:
	go test -v -cover ./...

testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${KERNEL}_${ARCH}
	cp bin/terraform-provider-daytona ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${KERNEL}_${ARCH}/

docs:
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	$(shell go env GOPATH)/bin/tfplugindocs generate

fmt:
	go fmt ./...

lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

tools:
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

clean:
	rm -rf bin/ dist/
