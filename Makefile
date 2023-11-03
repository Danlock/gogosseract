#! /usr/bin/make
SHELL = /bin/bash
BUILDTIME = $(shell date -u --rfc-3339=seconds)
GITHASH = $(shell git describe --dirty --always)
GITCOMMITNO = $(shell git rev-list --all --count)
SHORTBUILDTAG = v0.0.$(GITCOMMITNO)-$(GITHASH)
BUILDINFO = Build Time:$(BUILDTIME)
LDFLAGS = -X 'main.buildTag=$(SHORTBUILDTAG)' -X 'main.buildInfo=$(BUILDINFO)'

COVERAGE_PATH ?= .coverage

depend: deps
deps:
	go get ./...
	go mod tidy

version:
	@echo $(SHORTBUILDTAG)

unit-test:
	@go test -failfast -race -count=3 ./...

test:
	@go test -failfast -v -race -count=2 ./...

bench:
	@go test -failfast -benchmem -run=^$ -v -count=2 -bench .  ./...

recompile: tesseract-wasm/
	git submodule update --init --recursive
	cd tesseract-wasm/ && $(MAKE) docker-build
	cp --remove-destination tesseract-wasm/dist/tesseract-core.wasm internal/wasm/tesseract-core.wasm
#	 $(MAKE) gen

gen: internal/wasm/tesseract-core.wasm
	@go generate ./...

coverage:
	@go test -failfast -covermode=count -coverprofile=$(COVERAGE_PATH)

coverage-html:
	@rm $(COVERAGE_PATH) || true
	@$(MAKE) coverage
	@rm $(COVERAGE_PATH).html || true
	@go tool cover -html=$(COVERAGE_PATH) -o $(COVERAGE_PATH).html

coverage-browser:
	@rm $(COVERAGE_PATH) || true
	@$(MAKE) coverage
	@go tool cover -html=$(COVERAGE_PATH)

update-readme-badge:
	@go tool cover -func=$(COVERAGE_PATH) -o=$(COVERAGE_PATH).badge
	@go run github.com/AlexBeauchemin/gobadge@v0.3.0 -filename=$(COVERAGE_PATH).badge

# pkg.go.dev documentation is updated via go get updating the google proxy
update-godocs:
	@cd ../rmq; \
	GOPROXY=https://proxy.golang.org go get -u github.com/danlock/gogosseract; \
	go mod tidy

release:
	@$(MAKE) deps
ifeq ($(findstring dirty,$(SHORTBUILDTAG)),dirty)
	@echo "Version $(SHORTBUILDTAG) is filthy, commit to clean it" && exit 1
endif
	@read -t 5 -p "$(SHORTBUILDTAG) will be the new released version. Hit enter to proceed, CTRL-C to cancel."
	@$(MAKE) test
	@$(MAKE) bench
	@git tag $(SHORTBUILDTAG)
	@git push origin $(SHORTBUILDTAG)
