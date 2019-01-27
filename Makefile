export PATH := $(GOPATH)/bin:$(PATH)

version = 0.0.1
app = slex

fmt:
	gofmt -w .

test:
	go get ./...
	go test --cover ./...

build:
	rm -fr build 
	mkdir build 
	mkdir build/${app}-${version}-darwin 
	mkdir build/${app}-${version}-linux 
	mkdir build/${app}-${version}-win-386 
	mkdir build/${app}-${version}-win-amd64 
	env GOOS=darwin go build -o build/${app}-${version}-darwin/${app} ./cmd/slex 
	env GOOS=linux go build -o build/${app}-${version}-linux/${app} ./cmd/slex 
	env GOOS=windows GOARCH=amd64 go build -o build/${app}-${version}-win-amd64/${app}.exe ./cmd/slex 
	env GOOS=windows GOARCH=386 go build -o build/${app}-${version}-win-386/${app}.exe ./cmd/slex 
	cp conf/*.yaml build/${app}-${version}-darwin 
	cp conf/*.yaml build/${app}-${version}-linux 
	cp conf/*.yaml build/${app}-${version}-win-amd64 
	cp conf/*.yaml build/${app}-${version}-win-386
	cd build && zip -rq ${app}-${version}-darwin.zip ${app}-${version}-darwin
	cd build && zip -rq ${app}-${version}-linux.zip ${app}-${version}-linux
	cd build && zip -rq ${app}-${version}-win-amd64.zip ${app}-${version}-win-amd64
	cd build && zip -rq ${app}-${version}-win-386.zip ${app}-${version}-win-386

.PHONY: build