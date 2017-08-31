FROM golang
ADD . /go/src/github.com/kiyor/stf
RUN cd /go/src/github.com/kiyor/stf && \
	go get && \
	go install github.com/kiyor/stf

EXPOSE 30000
ENTRYPOINT ["/go/bin/stf"]
CMD ["-notimeout"]
