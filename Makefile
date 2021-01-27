GO ?= go

webcat:
	$(GO) build $(GOFLAGS) -o $@ .

.PHONY: webcat
