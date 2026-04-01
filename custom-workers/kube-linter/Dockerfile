# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT

###############
# Build stage #
###############
FROM golang:1.25-alpine AS builder
WORKDIR /go/src/app

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/kube-linter-worker .

# Install kube-linter CLI
RUN go install golang.stackrox.io/kube-linter/cmd/kube-linter@latest

#################
# Runtime stage #
#################
FROM alpine:latest

# Run as non-root
RUN addgroup -g 3000 appgroup && adduser -u 1000 -g appgroup --disabled-password --no-create-home appuser
USER 1000:3000

WORKDIR /app
COPY --from=builder /go/bin/kube-linter-worker .
COPY --from=builder /go/bin/kube-linter /usr/local/bin/kube-linter

ENTRYPOINT ["/app/kube-linter-worker"]
