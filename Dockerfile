FROM golang as builder
COPY . /go/src/github.com/kiyor/stf
RUN cd /go/src/github.com/kiyor/stf && \
    go get && \
    go build

FROM alpine
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY . .
COPY --from=builder /go/src/github.com/kiyor/stf/stf .
EXPOSE 30000 
ENTRYPOINT ["./stf","-notimeout"]
