#!/usr/bin/env bash

set -euo pipefail

REGISTRY="${REGISTRY:-8.130.70.64:5000}"
IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-miaozao}"
IMAGE_TAG="${IMAGE_TAG:-}"
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
  PUSH_LATEST      Also push :latest, default: true
  SERVICES         Services to build and push, default: "runtime"
  VERSION_FILE     Version file path, default: public/miaozao-version.txt

Examples:
  /data/miaozao/scripts/push-docker-images.sh
  IMAGE_TAG=release-20260822 /data/miaozao/scripts/push-docker-images.sh
  SERVICES=runtime /data/miaozao/scripts/push-docker-images.sh
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
        runtime)
            printf '%s\n' "${REPO_ROOT}"
            ;;
        *)
            echo "unsupported service: $1" >&2
            echo "supported services: runtime" >&2
            exit 1
            ;;
    esac
}

image_dockerfile() {
    case "$1" in
        runtime)
            printf '%s\n' "Dockerfile"
            ;;
        *)
            echo "unsupported service: $1" >&2
            echo "supported services: runtime" >&2
            exit 1
            ;;
    esac
}

build_and_push() {
    local service="$1"
    local context
    local dockerfile
    local image
    context="$(image_context "${service}")"
    dockerfile="$(image_dockerfile "${service}")"
    image="${REGISTRY}/${IMAGE_NAMESPACE}/${service}"

    echo "Building ${image}:${IMAGE_TAG} from ${context} using ${dockerfile}"
    if [ "${PUSH_LATEST}" = "true" ]; then
        docker build -f "${context}/${dockerfile}" -t "${image}:${IMAGE_TAG}" -t "${image}:latest" "${context}"
        docker push "${image}:${IMAGE_TAG}"
        docker push "${image}:latest"
    else
        docker build -f "${context}/${dockerfile}" -t "${image}:${IMAGE_TAG}" "${context}"
        docker push "${image}:${IMAGE_TAG}"
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

mkdir -p "$(dirname "${VERSION_FILE}")"
printf '%s\n' "${IMAGE_TAG}" > "${VERSION_FILE}"

echo "Registry: ${REGISTRY}"
echo "Namespace: ${IMAGE_NAMESPACE}"
echo "Image tag: ${IMAGE_TAG}"
echo "Push latest: ${PUSH_LATEST}"
echo "Services: ${SERVICES}"
echo "Version file: ${VERSION_FILE}"

for service in ${SERVICES}; do
    build_and_push "${service}"
done

echo
echo "Pushed images:"
for service in ${SERVICES}; do
    echo "  ${REGISTRY}/${IMAGE_NAMESPACE}/${service}:${IMAGE_TAG}"
    if [ "${PUSH_LATEST}" = "true" ]; then
        echo "  ${REGISTRY}/${IMAGE_NAMESPACE}/${service}:latest"
    fi
done
