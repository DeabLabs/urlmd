# Stage 1: Modules caching
FROM --platform=$BUILDPLATFORM golang:1.23-bullseye AS modules
COPY go.mod go.sum /modules/
WORKDIR /modules
RUN go mod download

# Stage 2: Build
FROM --platform=$BUILDPLATFORM golang:1.23-bullseye AS builder
# Copy cached modules
COPY --from=modules /go/pkg /go/pkg
COPY . /app
WORKDIR /app

# Install playwright CLI with correct version
RUN PWGO_VER=$(grep -oE "playwright-go v\S+" /app/go.mod | sed 's/playwright-go //g') \
  && go install github.com/playwright-community/playwright-go/cmd/playwright@${PWGO_VER}

# Build the application with proper architecture
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go build -o /bin/urlmd cmd/api/main.go

# Stage 3: Final
FROM --platform=$TARGETPLATFORM debian:bullseye-slim
WORKDIR /app

# Copy the binary and playwright
COPY --from=builder /bin/urlmd /app/
COPY --from=builder /go/bin/playwright /app/

# Install only necessary dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  tzdata \
  fonts-liberation \
  libasound2 \
  libatk-bridge2.0-0 \
  libatk1.0-0 \
  libatspi2.0-0 \
  libcairo2 \
  libcups2 \
  libdbus-1-3 \
  libdrm2 \
  libexpat1 \
  libgbm1 \
  libglib2.0-0 \
  libnspr4 \
  libnss3 \
  libpango-1.0-0 \
  libx11-6 \
  libxcb1 \
  libxcomposite1 \
  libxdamage1 \
  libxext6 \
  libxfixes3 \
  libxrandr2 \
  xdg-utils \
  && /app/playwright install --with-deps chromium \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* \
  && rm -rf /var/cache/apt/archives/*

ENV PATH="/app:${PATH}"
WORKDIR /app
CMD ["./urlmd"]