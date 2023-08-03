FROM golang:bookworm AS build
WORKDIR /app
COPY go.mod ./
COPY --link go.sum ./
RUN go mod download
COPY --link *.go ./
RUN CGO_ENABLED=0 go build -o api

FROM gcr.io/distroless/static-debian11:nonroot
LABEL authors="eve0415"
COPY --link --from=build /app/api /
EXPOSE 8080
CMD ["/api"]
