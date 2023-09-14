OBJECTS = controller \
		  worker \
		  job_manager


## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' Makefile | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	git diff --exit-code


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go test -race -buildvcs -vet=off ./...

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	go test -v -race -buildvcs -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out

## build: build the application
.PHONY: build
build: $(OBJECTS)
	
controller:
	GOARCH=amd64 GOOS=linux go build -o=./bin/start_controller-amd64 ./cmd/controller/start_controller.go
	GOARCH=arm64 GOOS=linux go build -o=./bin/start_controller-arm64 ./cmd/controller/start_controller.go

worker:
	GOARCH=amd64 GOOS=linux go build -o=./bin/start_worker-amd64 ./cmd/worker/start_worker.go
	GOARCH=arm64 GOOS=linux go build -o=./bin/start_worker-arm64 ./cmd/worker/start_worker.go

job_manager:
	GOARCH=amd64 GOOS=linux go build -o=./bin/job_manager-amd64 ./cmd/job_manager/job_manager.go
	GOARCH=arm64 GOOS=linux go build -o=./bin/job_manager-arm64 ./cmd/job_manager/job_manager.go

## clean: clean built files
.PHONY: clean
clean:
	go clean
	rm bin/*