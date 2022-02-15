FROM node:16.14-bullseye-slim

WORKDIR /app
ADD package.json .
RUN yarn install
ADD . .
