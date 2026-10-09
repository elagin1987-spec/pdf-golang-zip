# Pinned tool builds plus probe, QA and HTTP runtime targets.
FROM golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61 AS go-base

FROM ubuntu:24.04@sha256:534baea6a22c03a63003dbc8dbe78fe34bc0d7e595d9a9dc9834884ff530eb55 AS ubuntu-base
# The bare Ubuntu image has no CA bundle; bootstrap it from the pinned Go image.
COPY --from=go-base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV APT_SNAPSHOT=20261008T000000Z

FROM ubuntu-base AS tools-build
RUN apt-get -o APT::Update::Error-Mode=any --snapshot "$APT_SNAPSHOT" update && apt-get --snapshot "$APT_SNAPSHOT" install -y --no-install-recommends \
    ca-certificates curl build-essential cmake pkg-config libjpeg-dev zlib1g-dev \
    libssl-dev libfontconfig1-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN curl -fL https://github.com/ArtifexSoftware/ghostpdl-downloads/releases/download/gs10080/ghostscript-10.08.0.tar.xz -o gs.tar.xz \
    && echo 'c20492bc8ebb96c87fa2e52a0926e1cda8cde95d66145e018ac713fed5da38cf  gs.tar.xz' | sha256sum -c - \
    && curl -fL https://github.com/qpdf/qpdf/releases/download/v12.4.2/qpdf-12.4.2.tar.gz -o qpdf.tar.gz \
    && echo '8a58af5b6141319287c1883bec8bd1bd545b7567b7fc5e6ce5d25a1c85f36397  qpdf.tar.gz' | sha256sum -c - \
    && tar -xf gs.tar.xz && tar -xf qpdf.tar.gz
WORKDIR /src/ghostscript-10.08.0
RUN ./configure --prefix=/opt/pdf-tools --without-x --disable-cups --disable-gtk --without-tesseract \
    && make -j2 && make install \
    && mkdir -p /opt/pdf-tools/share/doc/ghostscript \
    && cp LICENSE doc/COPYING /opt/pdf-tools/share/doc/ghostscript/ \
    && cp config.log /opt/pdf-tools/share/doc/ghostscript/build-config.log
WORKDIR /src/qpdf-12.4.2
RUN cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/opt/pdf-tools \
    -DBUILD_SHARED_LIBS=ON -DBUILD_STATIC_LIBS=OFF -DBUILD_TESTING=OFF -DBUILD_DOC=OFF \
    && cmake --build build -j2 && cmake --install build \
    && mkdir -p /opt/pdf-tools/share/doc/qpdf \
    && cp LICENSE.txt build/CMakeCache.txt /opt/pdf-tools/share/doc/qpdf/
RUN mkdir -p /opt/pdf-tools/share/doc/sources \
    && cp /src/gs.tar.xz /src/qpdf.tar.gz /opt/pdf-tools/share/doc/sources/ \
    && dpkg-query -W > /opt/pdf-tools/share/doc/sources/build-packages.txt

FROM go-base AS go-build
WORKDIR /src
COPY go.mod ./
COPY internal/ internal/
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /engine-probe ./cmd/engine-probe \
    && CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /server ./cmd/server \
    && CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /pdfworker ./cmd/pdfworker

FROM ubuntu-base AS probe
RUN apt-get -o APT::Update::Error-Mode=any --snapshot "$APT_SNAPSHOT" update && apt-get --snapshot "$APT_SNAPSHOT" install -y --no-install-recommends \
    libfontconfig1 libjpeg-turbo8 zlib1g libssl3t64 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=tools-build /opt/pdf-tools /opt/pdf-tools
ENV PATH="/opt/pdf-tools/bin:${PATH}" LD_LIBRARY_PATH="/opt/pdf-tools/lib" XDG_CACHE_HOME="/tmp/cache"
COPY --from=go-build /engine-probe /usr/local/bin/engine-probe
COPY --from=go-base /usr/local/go/LICENSE /usr/share/doc/pdf-tools/GO-LICENSE
COPY THIRD_PARTY_NOTICES.md /usr/share/doc/pdf-tools/THIRD_PARTY_NOTICES.md
COPY testdata/pdf/ /fixtures/
RUN mkdir -p /work/jobs && chmod 1777 /work/jobs \
    && dpkg-query -W > /usr/share/doc/pdf-tools/runtime-packages.txt \
    && sha256sum /opt/pdf-tools/bin/gs /opt/pdf-tools/bin/qpdf /usr/local/bin/engine-probe > /usr/share/doc/pdf-tools/binaries.sha256
USER 65532:65532
WORKDIR /
ENTRYPOINT ["engine-probe"]
CMD ["-qa=false", "-out", "/work/jobs/stage-01"]

FROM probe AS qa
USER root
RUN apt-get -o APT::Update::Error-Mode=any --snapshot "$APT_SNAPSHOT" update && apt-get --snapshot "$APT_SNAPSHOT" install -y --no-install-recommends poppler-utils python3-pil \
    && rm -rf /var/lib/apt/lists/* \
    && dpkg-query -W > /usr/share/doc/pdf-tools/qa-packages.txt
USER 65532:65532

CMD ["-qa=true", "-out", "/work/jobs/stage-01"]

FROM probe AS runtime
USER root
COPY --from=go-build /server /usr/local/bin/server
COPY --from=go-build /pdfworker /usr/local/bin/pdfworker
RUN rm -rf /fixtures \
    && sha256sum /usr/local/bin/server /usr/local/bin/pdfworker >> /usr/share/doc/pdf-tools/binaries.sha256
ENV LISTEN_ADDR=":8080" TEMP_DIR="/work/jobs" WORKER_PATH="/usr/local/bin/pdfworker"
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["server"]
CMD []

# Test harness only: same runtime tools and binaries, with Go for local lifecycle tests.
FROM runtime AS api-test
USER root
COPY --from=go-base /usr/local/go /usr/local/go
COPY . /src
ENV PATH="/usr/local/go/bin:${PATH}" GOCACHE="/tmp/go-cache" GOPATH="/tmp/go-path" GOFLAGS="-buildvcs=false" PDF_SERVER_BINARY="/usr/local/bin/server"
USER 65532:65532
WORKDIR /src
ENTRYPOINT ["go"]
CMD ["test", "-v", "-count=1", "./tests/e2e"]

# Stage 07 only: independent decoder/symbol QA, never included in runtime.
FROM go-base AS quality-build
WORKDIR /src
COPY go.mod ./
COPY internal/ internal/
COPY scripts/quality-profiles/ scripts/quality-profiles/
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /quality-profiles ./scripts/quality-profiles

FROM qa AS quality-test
USER root
RUN apt-get -o APT::Update::Error-Mode=any --snapshot "$APT_SNAPSHOT" update && apt-get --snapshot "$APT_SNAPSHOT" install -y --no-install-recommends zbar-tools \
    && rm -rf /var/lib/apt/lists/*
COPY --from=go-build /server /usr/local/bin/server
COPY --from=go-build /pdfworker /usr/local/bin/pdfworker
COPY --from=quality-build /quality-profiles /usr/local/bin/quality-profiles
COPY --from=go-base /usr/local/go /usr/local/go
COPY . /src
ENV PATH="/usr/local/go/bin:${PATH}" GOCACHE="/tmp/go-cache" GOPATH="/tmp/go-path" GOFLAGS="-buildvcs=false" PDF_SERVER_BINARY="/usr/local/bin/server"
USER 65532:65532
WORKDIR /src
ENTRYPOINT ["python3", "/src/scripts/quality_load.py"]
