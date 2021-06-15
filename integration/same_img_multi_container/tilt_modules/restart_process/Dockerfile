FROM alpine/git

RUN apk update && apk add make
RUN apk add build-base

RUN git clone https://github.com/eradman/entr.git /entr
WORKDIR /entr
RUN git checkout c564e6bdca1dfe2177d1224363cad734158863ad
RUN ./configure; CFLAGS="-static" make install

FROM scratch

COPY --from=0  /usr/local/bin/entr /

ADD tilt-restart-wrapper /
