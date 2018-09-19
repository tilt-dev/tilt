# iron/go:dev is the alpine image with the go tools added
FROM iron/go:dev as builder

WORKDIR /app

RUN apk add --no-cache curl  # just for easy testing of endpoints, can remove later

ENTRYPOINT ["./server"]
