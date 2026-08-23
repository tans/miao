#!/usr/bin/env bash

set -euo pipefail

REGISTRY="${REGISTRY:-8.130.70.64:5000}"
IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-miaozao}"
IMAGE_TAG="${IMAGE_TAG:-}"
BUN_VERSION="${BUN_VERSION:-1.3.6}"
BUN_BASE_URL="${BUN_BASE_URL:-https://cdn.npmmirror.com/binaries/bun}"
DEBIAN_BASE="${DEBIAN_BASE:-public.ecr.aws/docker/library/debian:bookworm-slim}"
APT_MIRROR="${APT_MIRROR:-mirrors.tuna.tsinghua.edu.cn}"
PUSH_LATEST="${PUSH_LATEST:-true}"
SERVICES="${SERVICES:-runtime}"
VERSION_FILE="${VERSION_FILE:-public/miaozao-version.txt}"

usage() {
    cat <<EOF
Usage:
  /data/miaozao/scripts/push-docker-images.sh

Environment variables:
  REGISTRY          Docker registry, default: 8.130.70.64:5000
  IMAGE_NAMESPACE  Image namespace, default: miaozao
  IMAGE_TAG        Image tag, default: current time like 20260822-165230
  BUN_VERSION      Bun version for the base image, default: 1.3.6
  BUN_BASE_URL     Bun binary mirror, default: https://cdn.npmmirror.com/binaries/bun
  DEBIAN_BASE      Debian base image, default: public.ecr.aws/docker/library/debian:bookworm-slim
  APT_MIRROR       apt mirror host, default: mirrors.tuna.tsinghua.edu.cn
  PUSH_LATEST      Also push :latest, default: true
  SERVICES         Services to build and push, default: "runtime"
                   "runtime" always builds miaozao/bun-base first.
  VERSION_FILE     Version file path, default: public/miaozao-version.txt

Examples:
  /data/miaozao/scripts/push-docker-images.sh
  IMAGE_TAG=release-20260822 /data/miaozao/scripts/push-docker-images.sh
  SERVICES=runtime /data/miaozao/scripts/push-docker-images.sh
  SERVICES=bun-base BUN_VERSION=1.3.6 /data/miaozao/scripts/push-docker-images.sh
EOF
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "missing command: $1" >&2
        exit 1
    fi
}

image_context() {
    case "$1" in
        runtime|bun-base)
            printf '%s\n' "${REPO_ROOT}"
            ;;
        *)
            echo "unsupported service: $1" >&2
            echo "supported services: runtime bun-base" >&2
            exit 1
            ;;
    esac
}

image_dockerfile() {
    case "$1" in
        runtime)
            printf '%s\n' "Dockerfile"
            ;;
        bun-base)
            printf '%s\n' "Dockerfile.bun-base"
            ;;
        *)
            echo "unsupported service: $1" >&2
            echo "supported services: runtime bun-base" >&2
            exit 1
            ;;
    esac
}

build_and_push() {
    local service="$1"
    local context
    local dockerfile
    local image
    local tag="${IMAGE_TAG}"
    local build_args=()
    local tags=()

    context="$(image_context "${service}")"
    dockerfile="$(image_dockerfile "${service}")"
    image="${REGISTRY}/${IMAGE_NAMESPACE}/${service}"

    case "${service}" in
        bun-base)
            tag="${BUN_VERSION}"
            build_args+=(--build-arg "BUN_VERSION=${BUN_VERSION}")
            build_args+=(--build-arg "BUN_BASE_URL=${BUN_BASE_URL}")
            build_args+=(--build-arg "DEBIAN_BASE=${DEBIAN_BASE}")
            build_args+=(--build-arg "APT_MIRROR=${APT_MIRROR}")
            # Keep a short local name so `docker compose` builds can reuse it.
            tags+=("-t" "miaozao/${service}:${BUN_VERSION}")
            if [ "${PUSH_LATEST}" = "true" ]; then
                tags+=("-t" "miaozao/${service}:latest")
            fi
            ;;
        runtime)
            build_args+=(--build-arg "BASE_IMAGE=${REGISTRY}/${IMAGE_NAMESPACE}/bun-base:${BUN_VERSION}")
            ;;
    esac

    tags+=("-t" "${image}:${tag}")
    if [ "${PUSH_LATEST}" = "true" ]; then
        tags+=("-t" "${image}:latest")
    fi

    echo "Building ${image}:${tag} from ${context} using ${dockerfile}"
    docker build -f "${context}/${dockerfile}" "${build_args[@]}" "${tags[@]}" "${context}"

    echo "Pushing ${image}:${tag}"
    docker push "${image}:${tag}"
    if [ "${PUSH_LATEST}" = "true" ]; then
        echo "Pushing ${image}:latest"
        docker push "${image}:latest"
    fi
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    usage
    exit 0
fi

require_cmd git
require_cmd docker

REPO_ROOT="$(git -C "$(dirname "${BASH_SOURCE[0]}")/.." rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

if [ -z "${IMAGE_TAG}" ]; then
    IMAGE_TAG="$(date +%Y%m%d-%H%M%S)"
fi

if [ -z "${SERVICES}" ]; then
    SERVICES="runtime"
fi

BUILD_SERVICES="${SERVICES}"
# runtime depends on miaozao/bun-base; ensure it is built first.
case " ${BUILD_SERVICES} " in
    *" runtime "*)
        if [[ " ${BUILD_SERVICES} " != *" bun-base "* ]]; then
            BUILD_SERVICES="bun-base ${BUILD_SERVICES}"
        fi
        ;;
esac

if [[ " ${BUILD_SERVICES} " == *" runtime "* ]]; then
    mkdir -p "$(dirname "${VERSION_FILE}")"
    printf '%s\n' "${IMAGE_TAG}" > "${VERSION_FILE}"
fi

echo "Registry: ${REGISTRY}"
echo "Namespace: ${IMAGE_NAMESPACE}"
echo "Image tag: ${IMAGE_TAG}"
echo "Bun version: ${BUN_VERSION}"
echo "Bun base url: ${BUN_BASE_URL}"
echo "Debian base: ${DEBIAN_BASE}"
echo "apt mirror: ${APT_MIRROR}"
echo "Push latest: ${PUSH_LATEST}"
echo "Services: ${SERVICES}"
echo "Build order: ${BUILD_SERVICES}"
echo "Version file: ${VERSION_FILE}"

for service in ${BUILD_SERVICES}; do
    build_and_push "${service}"
done

echo
echo "Pushed images:"
for service in ${BUILD_SERVICES}; do
    if [ "${service}" = "bun-base" ]; then
        echo "  ${REGISTRY}/${IMAGE_NAMESPACE}/${service}:${BUN_VERSION}"
    else
        echo "  ${REGISTRY}/${IMAGE_NAMESPACE}/${service}:${IMAGE_TAG}"
    fi
    if [ "${PUSH_LATEST}" = "true" ]; then
        echo "  ${REGISTRY}/${IMAGE_NAMESPACE}/${service}:latest"
    fi
done
