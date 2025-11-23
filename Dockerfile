FROM alpine AS cache

RUN apk add -U --no-cache ca-certificates


FROM node:latest as admin

WORKDIR /app
COPY ./admin/package.json .
COPY ./admin/pnpm-lock.yaml .
RUN npm install -g pnpm
RUN pnpm install
COPY ./admin .
RUN pnpm run build



FROM node:latest as frontend
WORKDIR /app

COPY ./ui/package.json .
COPY ./ui/package-lock.json .
RUN npm install
COPY ./ui .
RUN node ./build.js

FROM golang:alpine as backend
WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@latest
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY . .

COPY --from=admin /app/dist ./assets/js/admin
COPY --from=frontend /assets/js/pad/assets/pad.js ./assets/js/pad/assets/pad.js
COPY --from=frontend /assets/js/welcome/assets/welcome.js ./assets/js/welcome/assets/welcome.js


RUN templ generate
RUN go build -o app .


FROM scratch as runtime
EXPOSE 3000

ENV NODE_ENV=production
ENV ETHERPAD_SETTINGS_PATH=/
COPY --from=backend /app/app /app


COPY --from=cache /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app"]


