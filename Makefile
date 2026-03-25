
hx711-loadcell: *.go cmd/module/*.go go.mod go.sum
	GOOS=linux GOARCH=arm64 go build -o hx711-loadcell cmd/module/cmd.go


test:
	GOOS=linux GOARCH=arm64 go test

lint:
	gofmt -w -s .

module.tar.gz: hx711-loadcell
	tar czf module.tar.gz meta.json hx711-loadcell

module: module.tar.gz

all: module test

update:
	go get go.viam.com/rdk@latest
	go mod tidy
