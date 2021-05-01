FROM golang:1.16 as build

WORKDIR /go/src/github.com/webdevops/azure-resourcemanager-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-resourcemanager-exporter
COPY ./go.sum /go/src/github.com/webdevops/azure-resourcemanager-exporter
COPY ./Makefile /go/src/github.com/webdevops/azure-resourcemanager-exporter
RUN make dependencies

# Compile
COPY ./ /go/src/github.com/webdevops/azure-resourcemanager-exporter
RUN make test
RUN make lint
RUN make build
RUN ./azure-resourcemanager-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
ENV LOG_JSON=1
COPY --from=build /go/src/github.com/webdevops/azure-resourcemanager-exporter/azure-resourcemanager-exporter /
USER 1000
ENTRYPOINT ["/azure-resourcemanager-exporter"]
