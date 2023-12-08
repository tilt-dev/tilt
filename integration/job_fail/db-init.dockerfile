FROM busybox
ENTRYPOINT ["sh", "-c", "sleep 1 && echo 'db-init job failed' && exit 1"]

