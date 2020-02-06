FROM golang:1.13-alpine AS builder

ENV GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

RUN apk add make git
WORKDIR /src
COPY . .

RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates

RUN addgroup -g 1001 appgroup && \
  adduser -H -D -s /bin/false -G appgroup -u 1001 appuser

USER 1001:1001
COPY --from=builder /src/releases/harbor_exporter /bin/harbor_exporter
ENTRYPOINT ["/bin/harbor_exporter"]