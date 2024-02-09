# This docker file is just used for development and testing purposes
FROM docker.io/golang:1.21-alpine3.18 AS development
ARG arch=x86_64

# ENV CGO_ENABLED=0
WORKDIR /go/src/app/
COPY . /go/src/app/

RUN apk update && apk add --no-cache \
    llvm \
    clang \
    llvm-static \
    llvm-dev \
    make \
    libbpf \
    libbpf-dev \
    musl-dev

RUN mkdir -p /build/ && \
    make build && \
    cp ./bin/* /build/


#----------------------------#

FROM docker.io/alpine:3.18.4 AS production

RUN apk update && apk add iproute2 curl

WORKDIR /app/
COPY --from=development /build .

CMD ["./testwave"]