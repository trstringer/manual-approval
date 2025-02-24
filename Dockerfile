FROM alpine:3.21.3

RUN apk update && \
   apk add ca-certificates

COPY app /var/app/app

CMD ["/var/app/app"]
