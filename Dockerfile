FROM golang AS builder

WORKDIR /build

ADD . /build

RUN bash build.sh

FROM alpine:3.21 AS runner

RUN apk add --no-cache ca-certificates ffmpeg

WORKDIR /app

COPY --from=builder /build/Music163bot-Feishu .

CMD [ "./Music163bot-Feishu" ]
