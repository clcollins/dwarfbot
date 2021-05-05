FROM docker.io/library/golang:1.15 as builder

RUN mkdir /src
WORKDIR /src

COPY . /src

RUN go build -o out/dwarfbot
RUN out/dwarfbot --help


# Build the final image
FROM registry.access.redhat.com/ubi8/ubi-minimal
COPY --from=builder /src/out/dwarfbot /dwarfbot

USER 1000
ENTRYPOINT ["/dwarfbot"]
CMD ["--help"]