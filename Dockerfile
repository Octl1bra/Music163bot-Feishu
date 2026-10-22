FROM golang:1.27.3-trixie AS builder
ENV GOTOOLCHAIN=local

WORKDIR /build

ADD . /build

RUN go test -p 1 -parallel 1 ./... \
    && go vet -p 1 ./... \
    && bash build.sh \
    && go version -m ./Music163bot-Feishu | tee /tmp/buildinfo \
    && grep -q ': go1.27.3$' /tmp/buildinfo

FROM alpine:3.21 AS runner

RUN apk add --no-cache ca-certificates ffmpeg

WORKDIR /app

COPY --from=builder /build/Music163bot-Feishu .

CMD [ "./Music163bot-Feishu" ]
