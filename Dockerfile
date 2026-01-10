FROM debian:bookworm-slim AS cache

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*


FROM node:bookworm-slim AS admin

WORKDIR /app
COPY ./admin/package.json .
COPY ./admin/pnpm-lock.yaml .
RUN npm install -g pnpm
RUN pnpm install
COPY ./admin .
RUN pnpm run build



FROM node:bookworm-slim AS frontend
WORKDIR /app

RUN npm install -g pnpm

COPY ./admin /app/admin
WORKDIR /app/admin
RUN pnpm install

WORKDIR /app

COPY ./assets /app/assets
COPY ./ui/package.json /app/ui/
COPY ./ui/pnpm-lock.yaml /app/ui/
WORKDIR /app/ui
RUN pnpm install
COPY ./ui /app/ui
WORKDIR /app/ui
RUN node ./build.js

FROM golang:bookworm AS backend
WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@latest
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY . .

COPY --from=admin /app/dist ./assets/js/admin
COPY --from=frontend /app/assets/js/timeslider/assets/timeslider.js ./assets/js/timeslider/assets/timeslider.js
COPY --from=frontend /app/assets/js/pad/assets/pad.js ./assets/js/pad/assets/pad.js
COPY --from=frontend /app/assets/js/welcome/assets/welcome.js ./assets/js/welcome/assets/welcome.js
COPY --from=frontend /app/assets/css/build ./assets/css/build

RUN templ generate
RUN go build -o app .


FROM scratch AS runtime
EXPOSE 3000

ENV NODE_ENV=production
ENV ETHERPAD_SETTINGS_PATH=/
COPY --from=backend /app/app /app


COPY --from=cache /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app"]
