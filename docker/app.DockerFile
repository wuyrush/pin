# idea: use dedicated image with necessary Go toolchain to build the app's executable binary,
# then move the binary to a separated image for deployment. In doing so we use Docker's
# multi-stage builds feature

FROM golang:alpine as builder

WORKDIR /build

ADD . .

RUN GOOS=linux go build -v -o ./pin-server .

FROM alpine:latest as deployer

WORKDIR /pin

COPY --from=builder /build/pin-server .

ENV PATH /pin:$PATH

ENTRYPOINT ["pin-server"]
