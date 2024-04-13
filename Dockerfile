FROM golang:1.22-alpine AS builder
COPY . /var/app
WORKDIR /var/app
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:3.19
RUN apk update && apk add ca-certificates
COPY --from=builder /var/app/app /var/app/app
CMD ["/var/app/app"]
