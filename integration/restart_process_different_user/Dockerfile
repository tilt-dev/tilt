FROM busybox
RUN addgroup different && adduser -D -G different different 
USER different
WORKDIR /src
ADD . .
RUN cp source.txt compiled.txt
