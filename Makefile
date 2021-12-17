.DEFAULT_GOAL = build
extension = $(patsubst windows,.exe,$(filter windows,$(1)))
GO := go
PKG_NAME := mongodb_atlas_prometheus_exporter
PREFIX := .
DOCKER_REPO ?= masaruhoshi/$(PKG_NAME)
DOCKER_LINUX_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v6,linux/arm/v7
DOCKER_PLATFORMS ?= $(DOCKER_LINUX_PLATFORMS),windows/amd64
BUILDX_ACTION ?= --output=type=image,push=false
# we just load by default, as a "dry run"
# BUILDX_ACTION ?= --load
TAG_LATEST ?= latest
TAG_ALPINE ?= alpine

DEBUG ?= 0

COMMIT ?= `git rev-parse --short HEAD 2>/dev/null`
VERSION ?= `git describe --abbrev=0 --tags $(git rev-list --tags --max-count=1) 2>/dev/null | sed 's/v\(.*\)/\1/'`

REVISION =`git rev-parse --short HEAD`
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

COMMIT_FLAG := -X `go list ./version/version`.Revision=$(REVISION)
VERSION_FLAG := -X `go list ./version/version`.Version=$(VERSION)

GOOS ?= $(shell go version | sed 's/^.*\ \([a-z0-9]*\)\/\([a-z0-9]*\)/\1/')
GOARCH ?= $(shell go version | sed 's/^.*\ \([a-z0-9]*\)\/\([a-z0-9]*\)/\2/')

ifeq ($(DEBUG), 1)
	GCFLAGS=-gcflags="all=-N -l"
else
	GCFLAGS=
endif

ifeq ("$(TARGETVARIANT)","")
ifneq ("$(GOARM)","")
TARGETVARIANT := v$(GOARM)
endif
else
ifeq ("$(GOARM)","")
GOARM ?= $(subst v,,$(TARGETVARIANT))
endif
endif

clean:
	rm -Rf $(PREFIX)/bin/*

$(PREFIX)/bin/$(PKG_NAME)_%v5$(call extension,$(GOOS)): $(shell find $(PREFIX) -type f -name "*.go")
	GOOS=$(shell echo $* | cut -f1 -d-) GOARCH=$(shell echo $* | cut -f2 -d- ) GOARM=5 CGO_ENABLED=0 \
		$(GO) build \
			-ldflags "-w -s $(COMMIT_FLAG) $(VERSION_FLAG)" \
			-o $@ \
			.

$(PREFIX)/bin/$(PKG_NAME)_%v6$(call extension,$(GOOS)): $(shell find $(PREFIX) -type f -name "*.go")
	GOOS=$(shell echo $* | cut -f1 -d-) GOARCH=$(shell echo $* | cut -f2 -d- ) GOARM=6 CGO_ENABLED=0 \
		$(GO) build \
			-ldflags "-w -s $(COMMIT_FLAG) $(VERSION_FLAG)" \
			-o $@ \
			.

$(PREFIX)/bin/$(PKG_NAME)_%v7$(call extension,$(GOOS)): $(shell find $(PREFIX) -type f -name "*.go")
	GOOS=$(shell echo $* | cut -f1 -d-) GOARCH=$(shell echo $* | cut -f2 -d- ) GOARM=7 CGO_ENABLED=0 \
		$(GO) build \
			-ldflags "-w -s $(COMMIT_FLAG) $(VERSION_FLAG)" \
			-o $@ \
			.

$(PREFIX)/bin/$(PKG_NAME)_windows-%.exe: $(shell find $(PREFIX) -type f -name "*.go")
	GOOS=windows GOARCH=$* GOARM= CGO_ENABLED=0 \
		$(GO) build \
			-ldflags "-w -s $(COMMIT_FLAG) $(VERSION_FLAG)" \
			-o $@ \
			.

$(PREFIX)/bin/$(PKG_NAME)_%$(TARGETVARIANT)$(call extension,$(GOOS)): $(shell find $(PREFIX) -type f -name "*.go")
	GOOS=$(shell echo $* | cut -f1 -d-) GOARCH=$(shell echo $* | cut -f2 -d- ) GOARM=$(GOARM) CGO_ENABLED=0 \
		$(GO) build \
			-ldflags "-w -s $(COMMIT_FLAG) $(VERSION_FLAG)" \
			-o $@ \
			.

$(PREFIX)/bin/$(PKG_NAME)$(call extension,$(GOOS)): $(PREFIX)/bin/$(PKG_NAME)_$(GOOS)-$(GOARCH)$(TARGETVARIANT)$(call extension,$(GOOS))
	cp $< $@

build: $(PREFIX)/bin/$(PKG_NAME)_$(GOOS)-$(GOARCH)$(TARGETVARIANT)$(call extension,$(GOOS)) $(PREFIX)/bin/$(PKG_NAME)$(call extension,$(GOOS))

docker-multi: Dockerfile
	docker buildx build \
		--build-arg VCS_REF=$(COMMIT) \
		--build-arg PKG_NAME=$(PKG_NAME) \
		--platform $(DOCKER_PLATFORMS) \
		--tag $(DOCKER_REPO):$(TAG_LATEST) \
		--target release \
		$(BUILDX_ACTION) .
	docker buildx build \
		--build-arg VCS_REF=$(COMMIT) \
		--build-arg PKG_NAME=$(PKG_NAME) \
		--platform $(DOCKER_LINUX_PLATFORMS) \
		--tag $(DOCKER_REPO):$(TAG_ALPINE) \
		--target alpine \
		$(BUILDX_ACTION) .

lint:
	@golangci-lint run --verbose --max-same-issues=0 --max-issues-per-linter=0 --sort-results

ci-lint:
	@golangci-lint run --verbose --max-same-issues=0 --max-issues-per-linter=0 --sort-results --out-format=github-actions

.PHONY: clean test build lint ci-lint
.DELETE_ON_ERROR:
.SECONDARY: