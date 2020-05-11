build:
	go build -o rl main.go

run:
	SERVER_PORT=80 go run main.go
