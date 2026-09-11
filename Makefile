.PHONY: build install

build:
	go build -o prettylogs ./cmd/prettylogs

# go install honours GOBIN (~/go/bin here), which is where prettylogs is run from.
install:
	go install ./cmd/prettylogs