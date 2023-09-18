all: build

install: build ## builds dperf
	@echo "Installing dperf binary to '$(GOPATH)/bin/dperf'"
	@mkdir -p $(GOPATH)/bin && cp -f $(PWD)/dperf $(GOPATH)/bin/dperf
	@echo "Installation successful. To learn more, try \"dperf --help\"."

build: 
	@CGO_ENABLED=0 go build --ldflags "-s -w"

clean:
	@echo "Cleaning up all the generated files"
	@find . -name '*.test' | xargs rm -fv
	@find . -name '*~' | xargs rm -fv
