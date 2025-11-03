FROM alpine AS cache

RUN apk add -U --no-cache ca-certificates


FROM node:latest as frontend
WORKDIR /app

RUN  npm install -g pnpm@latest

COPY ./ui/package.json .
COPY ./ui/pnpm-lock.yaml .
RUN pnpm install
COPY ./ui .
RUN pnpm run build

FROM golang:alpine as backend
WORKDIR /app
RUN go install github.com/a-h/templ/cmd/templ@latest
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY . .

COPY --from=frontend /assets/js /assets/js

RUN templ generate
RUN go build -o app .


FROM scratch as runtime
EXPOSE 3000

ENV NODE_ENV=production

COPY --from=backend /app/app /app


COPY --from=cache /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app"]


