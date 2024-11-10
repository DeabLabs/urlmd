# dev command with hot reloading, using github.com/air-verse/air
dev:
	go run cmd/api/main.go

# build for production
build:
	go build -o bin/urlmd cmd/api/main.go

# clean up
clean:
	rm -rf bin/*