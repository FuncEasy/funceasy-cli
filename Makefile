.PHONY: all
build-darwin:
	GOOS=darwin GOARCH=amd64 GOPROXY=https://goproxy.io GO111MODULE=on \
	go build -o ./build/darwin/bundles/funceasy-cli -v -ldflags "-s -w" ./main.go
	zip -rj ./build/funceasy-cli-darwin-amd64.zip ./build/darwin/bundles
build-linux:
	GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.io GO111MODULE=on \
	go build -o ./build/linux/bundles/funceasy-cli -v -ldflags "-s -w" ./main.go
	zip -rj ./build/funceasy-cli-linux-amd64.zip ./build/linux/bundles
clean:
	rm -rf ./build/bin