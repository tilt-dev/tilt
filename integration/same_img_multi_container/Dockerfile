FROM golang:1.11-alpine
WORKDIR /go/src/github.com/windmilleng/integration/same_img_multi_container
ADD . .
RUN go install github.com/windmilleng/integration/same_img_multi_container
ENTRYPOINT ./start.sh /go/bin/same_img_multi_container
