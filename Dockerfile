FROM docker:27-cli AS docker-cli

FROM alpine:3.21

ARG TARGETARCH

COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker
COPY dist/elastic-fruit-runner-linux-${TARGETARCH} /usr/local/bin/elastic-fruit-runner

RUN apk add --no-cache ca-certificates tzdata \
    && chmod +x /usr/local/bin/elastic-fruit-runner

ENTRYPOINT ["elastic-fruit-runner"]
