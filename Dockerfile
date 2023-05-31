FROM scratch
MAINTAINER MinIO Development "dev@min.io"

COPY ./dperf /dperf

ENTRYPOINT ["/dperf"]
