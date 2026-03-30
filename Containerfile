FROM registry.access.redhat.com/ubi9/go-toolset:1.25 as builder

COPY . .

RUN mkdir -p out && go build -buildvcs=false -o out/dwarfbot
RUN out/dwarfbot --help


# Build the final image
FROM registry.access.redhat.com/ubi9/ubi-minimal

ARG BUILD_DATE="1970-01-01T00:00:00Z"
ARG VCS_REF="unknown"
ARG VERSION="dev"

LABEL org.opencontainers.image.title="dwarfbot" \
      org.opencontainers.image.description="DwarfBot - a multi-platform chat bot" \
      org.opencontainers.image.url="https://github.com/clcollins/dwarfbot" \
      org.opencontainers.image.source="https://github.com/clcollins/dwarfbot" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.vendor="clcollins" \
      org.opencontainers.image.licenses="MIT" \
      io.k8s.display-name="dwarfbot" \
      io.k8s.description="DwarfBot - a multi-platform chat bot" \
      is.collins.cluster.image.revision="${VCS_REF}" \
      is.collins.cluster.image.version="${VERSION}" \
      is.collins.cluster.image.created="${BUILD_DATE}" \
      is.collins.cluster.build.commit.id="${VCS_REF}" \
      is.collins.cluster.build.date="${BUILD_DATE}"

COPY --from=builder /opt/app-root/src/out/dwarfbot /dwarfbot

USER 1001
EXPOSE 8080
ENTRYPOINT ["/dwarfbot"]
CMD ["--help"]
