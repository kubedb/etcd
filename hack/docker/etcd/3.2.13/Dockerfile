FROM quay.io/coreos/etcd:v3.2.13

RUN set -x \
  && apk add --update --no-cache ca-certificates

COPY etcd-operator /usr/bin/


ENV STANDBY warm
ENV RESTORE false
ENV BACKUP_NAME LATEST
ENV PITR latest
ENV ARCHIVE_S3_PREFIX ""
ENV RESTORE_S3_PREFIX ""

ENTRYPOINT ["etcd-operator"]
CMD ["leader_election"]
