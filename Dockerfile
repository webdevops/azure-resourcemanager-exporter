FROM golang:1.15 as build

WORKDIR /go/src/github.com/webdevops/azure-resourcemanager-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-resourcemanager-exporter
COPY ./go.sum /go/src/github.com/webdevops/azure-resourcemanager-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/azure-resourcemanager-exporter
RUN make lint
RUN make build
RUN ./azure-resourcemanager-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /go/src/github.com/webdevops/azure-resourcemanager-exporter/azure-resourcemanager-exporter /
USER 1000
ENTRYPOINT ["/azure-resourcemanager-exporter"]
