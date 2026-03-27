FROM registry.access.redhat.com/ubi9/go-toolset:1.25 as builder

COPY . .

RUN mkdir -p out && go build -buildvcs=false -o out/dwarfbot
RUN out/dwarfbot --help


# Build the final image
FROM registry.access.redhat.com/ubi9/ubi-minimal
COPY --from=builder /opt/app-root/src/out/dwarfbot /dwarfbot

USER 1001
ENTRYPOINT ["/dwarfbot"]
CMD ["--help"]
