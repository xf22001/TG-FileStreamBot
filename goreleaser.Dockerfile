FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ARG TARGETOS
ARG TARGETARCH
COPY ${TARGETOS}/${TARGETARCH}/fsb /app/fsb
ENTRYPOINT ["/app/fsb", "run"]