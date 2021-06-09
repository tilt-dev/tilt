FROM node:12

WORKDIR /app

ADD web/package.json package.json
RUN yarn install
ADD web .
RUN yarn run build-storybook
ENTRYPOINT npx http-server /app/storybook-static --host 0.0.0.0 --port 9009
