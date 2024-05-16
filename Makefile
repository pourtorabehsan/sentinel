build:
	@go build -o bin/sentinel cmd/sentinel/main.go

run: build
	@./bin/sentinel
