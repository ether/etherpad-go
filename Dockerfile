FROM debian:bookworm-slim AS cache

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*


FROM golang:bookworm AS backend
WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@latest
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY . .

# Pre-built assets from CI
COPY ./assets/js/admin ./assets/js/admin
COPY ./assets/js/timeslider/assets/timeslider.js ./assets/js/timeslider/assets/timeslider.js
COPY ./assets/js/pad/assets/pad.js ./assets/js/pad/assets/pad.js
COPY ./assets/js/welcome/assets/welcome.js ./assets/js/welcome/assets/welcome.js
COPY ./assets/js/sheet/assets/sheet.js ./assets/js/sheet/assets/sheet.js
COPY ./assets/css/build ./assets/css/build

RUN templ generate
# Static: the runtime image is scratch, without a C library or loader, so a
# cgo build (the default in golang:bookworm) would not start there.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o app .


FROM alpine:3.24 AS busybox
# The shell's tools for looking around the container (issue 413), on
# purpose without network tools (wget, nc, ...): the server runs as root.
ARG SHELL_APPLETS="sh ls cat less head tail grep find vi env ps df du pwd echo test readlink mkdir cp mv rm chmod"
RUN apk add --no-cache busybox-static \
    && mkdir -p /rootfs/bin \
    && cp /bin/busybox.static /rootfs/bin/busybox \
    && for applet in $SHELL_APPLETS; do ln -s /bin/busybox /rootfs/bin/$applet; done \
    && ln -s /etherpad-go /rootfs/app

FROM scratch AS runtime
EXPOSE 3000

ENV NODE_ENV=production
ENV ETHERPAD_SETTINGS_PATH=/
# Named like in the README, and on PATH: `docker exec <container>
# etherpad-go config show`, or a shell (busybox) with `docker exec -it
# <container> sh` (busybox: sh, ls, cat, vi, ...). /app stays as a link for deployments that start /app.
COPY --from=backend /app/app /etherpad-go
COPY --from=busybox /rootfs /


COPY --from=cache /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV PATH="/:/bin"
ENTRYPOINT ["/etherpad-go"]
