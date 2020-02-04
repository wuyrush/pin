# idea: use dedicated image with necessary Go toolchain to build the app's executable binary,
# then move the binary to a separated image for deployment. This way we reduce the size of
# deployment image. In doing so we use Docker's multi-stage builds feature
FROM golang:alpine as builder

WORKDIR /build

# used when building Golang application images in region where google/golang.org sites are blocked, e.g., CHN
ENV GOPROXY https://goproxy.io

ADD . .

RUN GOOS=linux go build -v -o ./pin-server wuyrush.io/pin/server

FROM alpine:latest as server

WORKDIR /pin

COPY --from=builder /build/pin-server .

ADD ./server/templates ./templates

ENV PATH /pin:$PATH

ENTRYPOINT ["pin-server"]
