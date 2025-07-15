FROM golang:latest AS builder

WORKDIR /go/src/goapp

COPY go.* ./
RUN go mod download

COPY . .

ENV GOCACHE=/root/.cache/go-build

RUN --mount=type=cache,target="/root/.cache/go-build" CGO_ENABLED=0 GOOS=linux go build -v -o /goapp

FROM alpine:latest

WORKDIR /app

COPY --from=builder /goapp /app/goapp

COPY bin/ffmpeg-linux/ffmpeg /usr/local/bin/ffmpeg
RUN chmod +x /usr/local/bin/ffmpeg

RUN apt-get install libwebp-dev -y

RUN mkdir -p /app/files

EXPOSE 2112

CMD ["./goapp"]