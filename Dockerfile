FROM        quay.io/prometheus/busybox:glibc
COPY        data_exporter  /bin/data_exporter

EXPOSE      9399
ENTRYPOINT  [ "/bin/data_exporter" ]