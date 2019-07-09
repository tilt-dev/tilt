FROM rust:slim

WORKDIR /usr/src/app
COPY . .

RUN cargo build
CMD ./target/debug/hello_world
