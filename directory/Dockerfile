FROM --platform=linux/amd64 golang:1.19.1 as build

WORKDIR /app_go
COPY . .
RUN make build-directory

FROM --platform=linux/amd64 alpine:latest

# Link the libraries since alpine is using musl instead of gclib
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
WORKDIR /
COPY --from=build /app_go/dir_app ./app
CMD ["/app"]