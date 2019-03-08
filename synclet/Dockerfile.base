FROM "golang/1.12-alpine" as builder

WORKDIR /app

RUN apk add --no-cache curl  # just for easy testing of endpoints, can remove later

ENTRYPOINT ["./server"]
