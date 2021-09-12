FROM golang:1.16.7-alpine3.14 as builder

RUN apk add --update --no-cache make

WORKDIR /app

COPY . /app

RUN make build


FROM alpine:latest as runner

COPY --from=builder /app/bin ./app/

EXPOSE 80

CMD ["./app/test_app"]