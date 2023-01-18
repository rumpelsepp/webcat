GO ?= go

.PHONY: webcat
webcat:
	$(GO) build $(GOFLAGS) -o $@ .
