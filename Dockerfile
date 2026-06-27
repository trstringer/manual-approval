FROM golang:1.24@sha256:d2d2bc1c84f7e60d7d2438a3836ae7d0c847f4888464e7ec9ba3a1339a1ee804 AS builder
COPY . /var/app
WORKDIR /var/app
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40
LABEL org.opencontainers.image.source=https://github.com/trstringer/manual-approval
RUN apk update && apk add ca-certificates
COPY --from=builder /var/app/app /var/app/app
CMD ["/var/app/app"]
