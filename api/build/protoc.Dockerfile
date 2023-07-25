ARG BASE_IMAGE=alpine:3.17
ARG GOLANG_IMAGE=golang:1.20.6-alpine

FROM $GOLANG_IMAGE as golang

ARG PROTOC_GEN_GO_VERSION=v1.31.0
ARG PROTOC_GEN_GO_GRPC_VERSION=v1.1.0

WORKDIR /

RUN export GOBIN=/bin && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION} && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}

FROM $BASE_IMAGE

COPY --from=golang /bin/protoc-gen-go /bin/protoc-gen-go
COPY --from=golang /bin/protoc-gen-go-grpc /bin/protoc-gen-go-grpc

ARG PROTOC_VERSION=v23.4
ARG PROTOC_FILE=protoc-23.4-linux-x86_64.zip

RUN wget https://github.com/protocolbuffers/protobuf/releases/download/${PROTOC_VERSION}/${PROTOC_FILE} && \
    unzip ${PROTOC_FILE} -x '*.proto' -x '*.txt' && \
    rm -f ${PROTOC_FILE}