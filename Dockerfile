FROM golang:1.19 as builder

# Set the Current Working Directory inside the container
WORKDIR /build

# Build flag
ENV CGO_ENABLED=0 

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build main.go

# Add non-root user
RUN useradd -u 10001 nonroot

# Install CA certs
FROM alpine as certimage
RUN apk add --no-cache ca-certificates

# Minimal Prod Image
FROM scratch

# Copy binary & CA certs from builder.
COPY --from=builder /build/main /main
COPY --from=certimage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Use non-root user
COPY --from=builder /etc/passwd /etc/passwd
USER nonroot

# Run the binary
CMD  ["./main"]
