FROM golang:1.27 AS builder

ARG TARGETPLATFORM

COPY ./.git /app/.git
COPY ./build /app/build
COPY ./cmd /app/cmd
COPY ./dev-scripts /app/dev-scripts
COPY ./garbagecollect /app/garbagecollect
COPY ./handlers /app/handlers
COPY ./picoshare /app/picoshare
COPY ./random /app/random
COPY ./space /app/space
COPY ./store /app/store
COPY ./go.* /app/

WORKDIR /app

RUN TARGETPLATFORM="${TARGETPLATFORM}" \
    ./dev-scripts/build-backend "prod"

FROM scratch AS artifact
COPY --from=builder /app/bin/picoshare ./

# Litestream is not included: its 0.3 release line ships Go 1.21 and 2023
# dependencies with known vulnerabilities, and this deployment does not
# replicate to S3. docker-entrypoint refuses to start if LITESTREAM_BUCKET is set.
FROM alpine:3.24

# Apply Alpine security fixes released since the base image was published.
RUN apk upgrade --no-cache && apk add --no-cache bash

COPY --from=builder /app/bin/picoshare /app/picoshare
COPY ./docker-entrypoint /app/docker-entrypoint
COPY ./LICENSE /app/LICENSE

WORKDIR /app

ENTRYPOINT ["/app/docker-entrypoint"]
CMD ["-db", "/data/store.db"]
