FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /mine ./cmd/mine

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /mine /usr/local/bin/mine
EXPOSE 8080
ENTRYPOINT ["mine"]
