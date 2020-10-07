FROM node:12

WORKDIR /app

ADD web/package.json package.json
RUN yarn install
ADD web .
ENTRYPOINT yarn run storybook -h 0.0.0.0
