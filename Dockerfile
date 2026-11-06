# Multi-platform builder stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /build
COPY . .

RUN go mod download && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a -installsuffix cgo -ldflags="-w -s" -o stf .

# Final minimal image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -u 1000 stf

WORKDIR /app
COPY --from=builder /build/stf .

USER stf
EXPOSE 30000

ENTRYPOINT ["/app/stf", "-notimeout"]
