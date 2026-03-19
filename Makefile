build:
	go build -o ai-shell .
install:
	go install .
cover:
	go test -cover ./...
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
