FROM alpine AS cache

RUN apk add -U --no-cache ca-certificates


FROM node:alpine AS admin

WORKDIR /app
COPY ./admin/package.json .
COPY ./admin/pnpm-lock.yaml .
RUN npm install -g pnpm
RUN pnpm install
COPY ./admin .
RUN pnpm run build



FROM node:alpine AS frontend
WORKDIR /app

RUN npm install -g pnpm

COPY ./admin ./admin

RUN cd ./admin \
    && pnpm install \
    && cd ../

COPY ./assets /assets
COPY ./ui/package.json ./ui/
COPY ./ui/pnpm-lock.yaml ./ui/
RUN cd ./ui/ && pnpm install
COPY ./ui ./ui
RUN cd ./ui && node ./build.js

FROM golang:alpine AS backend
WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@latest
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY . .

COPY --from=admin /app/dist ./assets/js/admin
COPY --from=frontend /assets/js/pad/assets/pad.js ./assets/js/pad/assets/pad.js
COPY --from=frontend /assets/js/welcome/assets/welcome.js ./assets/js/welcome/assets/welcome.js
COPY --from=frontend /assets/css/build ./assets/css/build

RUN templ generate
RUN go build -o app .


FROM scratch AS runtime
EXPOSE 3000

ENV NODE_ENV=production
ENV ETHERPAD_SETTINGS_PATH=/
COPY --from=backend /app/app /app


COPY --from=cache /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app"]


