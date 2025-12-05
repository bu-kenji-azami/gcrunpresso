FROM gcr.io/distroless/static-debian12
LABEL maintainer="fujiwara <fujiwara.shunichiro@gmail.com>"

ARG TARGETOS
ARG TARGETARCH

COPY ./dist/ecspresso_${TARGETOS}_${TARGETARCH}_v*/ecspresso /usr/local/bin/ecspresso
ENTRYPOINT ["/usr/local/bin/ecspresso"]
