# syntax=docker/dockerfile:1
FROM docker.io/debian:trixie-slim

ARG DEBIAN_FRONTEND="noninteractive"
ARG DEBCONF_NONINTERACTIVE_SEEN="true"
ARG PYTHON_VERSION="3.11"

ENV PYTHONUNBUFFERED="1"
ENV UV_PYTHON_INSTALL_DIR="/opt/uv/python"
ENV UV_PYTHON_BIN_DIR="/usr/local/bin"
ENV LANG="C.UTF-8"
ENV LC_ALL="C.UTF-8"

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/local/bin/
RUN uv python install "${PYTHON_VERSION}" --default
COPY crosscheck.py /usr/local/bin/crosscheck.py
RUN chmod +x /usr/local/bin/crosscheck.py
RUN uv sync --script /usr/local/bin/crosscheck.py

# Mount the committed oracle fixtures at /work/oracle:
#   docker run --rm -v "$PWD/verification/oracle:/work/oracle" meso-reference-python
WORKDIR /work

ENTRYPOINT ["/usr/local/bin/crosscheck.py"]
