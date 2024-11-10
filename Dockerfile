# Stage 1: Modules caching
FROM golang:1.23 as modules
COPY go.mod go.sum /modules/
WORKDIR /modules
RUN go mod download

# Stage 2: Build
FROM --platform=linux/amd64 ubuntu:latest as builder
COPY --from=modules /go/pkg /go/pkg
COPY . /workdir
WORKDIR /workdir
# Install go 1.23
RUN apt-get update && apt-get install -y golang-1.23 ca-certificates
# Make `go` command available
ENV PATH="/usr/lib/go-1.23/bin:${PATH}"
ENV GOPATH="/go"
# Install playwright cli with right version for later use
RUN PWGO_VER=$(grep -oE "playwright-go v\S+" /workdir/go.mod | sed 's/playwright-go //g') \
  && go install github.com/playwright-community/playwright-go/cmd/playwright@${PWGO_VER}
# Build your app
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bin/urlmd cmd/api/main.go

# Stage 3: Final
FROM --platform=linux/amd64 ubuntu:latest
COPY --from=builder /go/bin/playwright /bin/urlmd /
RUN apt-get update && apt-get install -y ca-certificates tzdata \
  # Install dependencies and all browsers (or specify one)
  && /playwright install --with-deps chromium \
  && rm -rf /var/lib/apt/lists/*
CMD ["/urlmd"]
