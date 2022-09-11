FROM golang:alpine as build

RUN apk update && apk upgrade && apk add tar ca-certificates build-base

ENV GOPATH /go
RUN go version

WORKDIR /arm.com/smarter-device-management
COPY . .

RUN echo $PATH;export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go mod init arm.com/smarter-device-management && go mod tidy && go mod vendor && \
    CGO_ENABLED=0 go build -ldflags='-s -w -extldflags="-static"' .

FROM scratch

WORKDIR /root

COPY conf.yaml /root/config/conf.yaml
COPY --from=build /arm.com/smarter-device-management/smarter-device-management /usr/bin/smarter-device-management

CMD ["/usr/bin/smarter-device-management","-logtostderr=true","-v=0"]
