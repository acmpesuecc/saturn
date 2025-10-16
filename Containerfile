FROM docker.io/golang:1.25-alpine3.22 AS buildStage

WORKDIR /app

COPY types.go ./
COPY timer.go ./
COPY main.go ./
COPY go.mod ./
COPY go.sum ./

RUN go build -o saturn

FROM docker.io/alpine:3.20

WORKDIR /opt
COPY --from=buildStage /app/saturn /opt/saturn

ENTRYPOINT ["/opt/saturn"]
