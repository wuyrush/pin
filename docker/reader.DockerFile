# idea: use dedicated image with necessary Go toolchain to build the app's executable binary,
# then move the binary to a separated image for deployment. This way we reduce the size of
# deployment image. In doing so we use Docker's multi-stage builds feature
FROM golang:alpine as builder

WORKDIR /build

# used when building Golang application images in region where google/golang.org sites are blocked, e.g., CHN
# ENV GOPROXY https://goproxy.io

# warm the cache to speed up builds - (most) builds without adding new deps will benefit from this
ADD ./go.mod ./go.sum /build/
RUN GOOS=linux go mod download

COPY . .

RUN GOOS=linux go build -v -o ./pin-reader wuyrush.io/pin/reader

FROM alpine:latest as deployer

WORKDIR /pin

COPY --from=builder /build/pin-reader .

# ADD ./reader/templates ./templates/
# TODO: serve static assets at proxy level

ENTRYPOINT ["/pin/pin-reader"]
