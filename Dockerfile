## build Go executable
FROM golang:1.19-alpine as go-build

## copy entire local folder to container root directory
COPY ./ /mqtt-gateway/

WORKDIR /mqtt-gateway/cmd/gateway

## build
RUN go build -v

## deploy
FROM alpine:3.17.0

## copy executable from go-build container
COPY --from=go-build /mqtt-gateway/cmd/gateway/gateway /app/gateway

## http port 
EXPOSE 50000

## entrypoint is gateway
ENTRYPOINT ["/app/gateway", "-httpHost", ""]
