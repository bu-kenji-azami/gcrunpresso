FROM gcr.io/distroless/static-debian12
LABEL maintainer="fujiwara <fujiwara.shunichiro@gmail.com>"

ARG TARGETOS
ARG TARGETARCH

COPY ./dist/gcrunpresso_${TARGETOS}_${TARGETARCH}_v*/gcrunpresso /usr/local/bin/gcrunpresso
ENTRYPOINT ["/usr/local/bin/gcrunpresso"]
