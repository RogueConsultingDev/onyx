ARG GO_VERSION=1.27.3
ARG IMAGE_URL=cgr.dev/chainguard/wolfi-base
ARG IMAGE_SHA=sha256:f15fea2f49cb14dceaea491fcbde0d35e0c190126a744ac05a9838440d47d994 # 2026-10-22T11:33:01Z

# <== Build Image ==>
FROM golang:${GO_VERSION} AS build
WORKDIR /src

# Download dependencies as a separate step to take advantage of Docker's caching.
COPY go.mod go.sum ./

RUN --mount=type=ssh \
    --mount=type=cache,target=/go/pkg/mod/ \
    go mod download;

# Copy the source files.
# Using Dockers wildcard mechanism to include directories if they exist.
COPY /cm[d] ./cmd
COPY /interna[l] ./internal
COPY /pk[g] ./pkg

# Build the application.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    GOCACHE=/go/pkg/mod/ go build -o /usr/local/bin/ ./...;


# <== Application Image ==>
FROM ${IMAGE_URL}@${IMAGE_SHA} AS app

# Copy the executable from the "build" stage.
COPY --from=build /usr/local/bin/* /usr/local/bin/
