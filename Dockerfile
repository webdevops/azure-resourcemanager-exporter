FROM golang:1.13 as build

WORKDIR /go/src/github.com/webdevops/azure-resourcemanager-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-resourcemanager-exporter
COPY ./go.sum /go/src/github.com/webdevops/azure-resourcemanager-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/azure-resourcemanager-exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o /azure-resourcemanager-exporter \
    && chmod +x /azure-resourcemanager-exporter
RUN /azure-resourcemanager-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /azure-resourcemanager-exporter /
USER 1000
ENTRYPOINT ["/azure-resourcemanager-exporter"]
