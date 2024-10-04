FROM golang:1.23 AS builder
COPY . /var/app
WORKDIR /var/app
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:3.20
LABEL org.opencontainers.image.source=https://github.com/trstringer/manual-approval
RUN apk update && apk add ca-certificates
COPY --from=builder /var/app/app /var/app/app
CMD ["/var/app/app"]
