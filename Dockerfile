FROM alpine AS cache

RUN apk add -U --no-cache ca-certificates


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

COPY --from=frontend /assets/js/pad /assets/js/pad
COPY --from=frontend /assets/js/welcome /assets/js/welcome


RUN templ generate
RUN go build -o app .


FROM scratch as runtime
EXPOSE 3000

ENV NODE_ENV=production
ENV ETHERPAD_SETTINGS_PATH=/
COPY --from=backend /app/app /app


COPY --from=cache /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app"]


