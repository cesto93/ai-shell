build:
	go build -o ai-shell .
install:
	go install .
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
