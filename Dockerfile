FROM golang:1.17.1-alpine as builder
LABEL maintainer="VikeLabs <gatekeeper@vikelabs.ca>"

RUN apk update && \
    apk add ca-certificates && \
    rm -rf /var/cache/apk/* && \
    update-ca-certificates
WORKDIR /app
COPY go.mod go.sum ./

RUN go mod download
# copy source code
COPY . .
# build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM scratch
WORKDIR /root/

# Copy the Pre-built binary file from the build stage
COPY --from=builder /app/main .
# Copy the certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

CMD ["./main"]