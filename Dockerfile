# syntax=docker/dockerfile:1.4
# This is the first stage, for building things that will be required by the
# final stage (notably the binary)
FROM golang:1.21 AS builder

ENV GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  GOFLAGS="-ldflags=-s -ldflags=-w"

WORKDIR /app

# Copy in just the go.mod and go.sum files, and download the dependencies. By
# doing this before copying in the other dependencies, the Docker build cache
# can skip these steps so long as neither of these two files change.
COPY go.mod go.sum ./
RUN go mod download && \
  go mod verify

# Assuming the source code is collocated to this Dockerfile
COPY main.go ./main.go

RUN go build -o /app/vault-unsealer

# Create a "nobody" non-root user for the next image by crafting an /etc/passwd
# file that the next image can copy in. This is necessary since the next image
# is based on scratch, which doesn't have adduser, cat, echo, or even sh.
RUN echo "nobody:x:65534:65534:nobody:/nonexistent:" > /minimal_passwd

# The second and final stage
FROM scratch

ENV TZ=Asia/Ho_Chi_Minh

WORKDIR /

# Copy the certs from the builder stage
COPY --from=builder --link /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copt timezone from the builder stage
COPY --from=builder --link /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the /etc/passwd file we created in the builder stage. This creates a new
# non-root user as a security best practice.
COPY --from=builder --link /minimal_passwd /etc/passwd

# Copy all of libraries of build stage to runtime stage to avoid unexpected errors
COPY --from=builder --link /lib/x86_64-linux-gnu /lib/x86_64-linux-gnu
COPY --from=builder --link /lib64/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2

COPY --from=builder /app/vault-unsealer /

USER nobody

ENTRYPOINT [ "/vault-unsealer" ]