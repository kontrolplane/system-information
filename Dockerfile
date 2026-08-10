FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . .

ARG VERSION=dev
ARG COMMIT=none

# -s -w strips the symbol table and DWARF; the binary is small on purpose.
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
  -o system-information ./cmd/system-information

FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/system-information /system-information

EXPOSE 9898
ENTRYPOINT ["/system-information"]
