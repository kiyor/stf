FROM golang as builder
COPY . /go/src/github.com/kiyor/stf
RUN cd /go/src/github.com/kiyor/stf && \
    go get && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o stf .

FROM alpine
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/src/github.com/kiyor/stf/stf .
EXPOSE 30000 
ENTRYPOINT ["/root/stf","-notimeout"]
