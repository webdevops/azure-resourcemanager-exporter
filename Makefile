PROJECT_NAME		:= $(shell basename $(CURDIR))
GIT_TAG				:= $(shell git describe --dirty --tags --always)
GIT_COMMIT			:= $(shell git rev-parse --short HEAD)
LDFLAGS				:= -X "main.gitTag=$(GIT_TAG)" -X "main.gitCommit=$(GIT_COMMIT)" -extldflags "-static" -s -w

FIRST_GOPATH			:= $(firstword $(subst :, ,$(shell go env GOPATH)))
GOLANGCI_LINT_BIN		:= $(FIRST_GOPATH)/bin/golangci-lint
GOSEC_BIN				:= $(FIRST_GOPATH)/bin/gosec

RELEASE_ASSETS = \
	$(foreach GOARCH,amd64 arm64,\
	$(foreach GOOS,linux darwin windows,\
		release-assets/$(GOOS).$(GOARCH))) \

word-dot = $(word $2,$(subst ., ,$1))

.PHONY: all
all: build

.PHONY: clean
clean:
	git clean -Xfd .

.PHONY: build
build:
	GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o $(PROJECT_NAME) .

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: image
image: build
	docker build -t $(PROJECT_NAME):$(GIT_TAG) .

build-push-development:
	docker buildx create --use
	docker buildx build -t webdevops/$(PROJECT_NAME):development --platform linux/amd64,linux/arm,linux/arm64 --push .

.PHONY: release-assets
release-assets: clean-release-assets vendor $(RELEASE_ASSETS)

clean-release-assets:
	rm -rf ./release-assets
	mkdir -p ./release-assets

release-assets/windows.%: $(SOURCE)
	echo 'build release-assets for windows/$(call word-dot,$*,2)'
	GOOS=windows \
 	GOARCH=$(call word-dot,$*,1) \
	CGO_ENABLED=0 \
	go build -ldflags '$(LDFLAGS)' -o './release-assets/$(PROJECT_NAME).windows.$(call word-dot,$*,1).exe' .

release-assets/%: $(SOURCE)
	echo 'build release-assets for $(call word-dot,$*,1)/$(call word-dot,$*,2)'
	GOOS=$(call word-dot,$*,1) \
 	GOARCH=$(call word-dot,$*,2) \
	CGO_ENABLED=0 \
	go build -ldflags '$(LDFLAGS)' -o './release-assets/$(PROJECT_NAME).$(call word-dot,$*,1).$(call word-dot,$*,2)' .

.PHONY: test
test:
	go test ./...

.PHONY: dependencies
dependencies:
	go mod vendor

.PHONY: check-release
check-release: vendor lint gosec test

.PHONY: lint
lint: $(GOLANGCI_LINT_BIN)
	$(GOLANGCI_LINT_BIN) run -E exportloopref,gofmt --timeout=30m

.PHONY: gosec
gosec: $(GOSEC_BIN)
	$(GOSEC_BIN) ./...

$(GOLANGCI_LINT_BIN):
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(FIRST_GOPATH)/bin

$(GOSEC_BIN):
	curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $(FIRST_GOPATH)/bin
