FROM alpine AS cache

RUN apk add -U --no-cache ca-certificates


FROM node:latest as frontend
WORKDIR /app

COPY ./ui/package.json .
COPY ./ui/package-lock.json .
RUN npm install
COPY ./ui .
RUN npm run build

FROM golang:alpine as backend
WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /assets/js/pad /assets/js/pad
RUN go install github.com/a-h/templ/cmd/templ@latest

RUN templ generate
RUN go build -o app .


FROM scratch as runtime
EXPOSE 3000

COPY --from=backend /app/app /app
COPY --from=frontend /assets /assets

COPY --from=cache /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app"]


