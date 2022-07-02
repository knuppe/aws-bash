VERSION=$(shell git describe --tags --always --long --dirty)

.PHONY: all clean

build: clean
	mkdir -p ./build
	env GOARCH=amd64 CGO_ENABLED=0 GOOS=windows go build -v -ldflags "-s -w -X main.version=$(VERSION)" -o ./build/aws-bash.exe
	env GOARCH=amd64 CGO_ENABLED=0 GOOS=linux   go build -v -ldflags "-s -w -X main.version=$(VERSION)" -o ./build/aws-bash

release: build
	upx --best ./build/aws-bash.exe
	upx --best ./build/aws-bash

clean:
	rm -rf ./build
