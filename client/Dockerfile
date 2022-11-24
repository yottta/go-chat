FROM --platform=linux/amd64 golang:1.19.1

WORKDIR /app_go
COPY . .
RUN make build-client

WORKDIR /
RUN cp /app_go/client_app ./app
CMD ["/app"]