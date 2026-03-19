build:
	go build -o ai-shell .
install:
	go install .
cover:
	go test -cover ./...
