#!/bin/bash
# =============================================================================
# WeKnora Sandbox 镜像构建脚本 (buildah 版本)
#
# 使用 buildah（无需 Docker daemon）构建 sandbox 镜像并推送到指定仓库。
# 适用于 Cube 集群无法访问 Docker Hub 的场景。
#
# 依赖: buildah (brew install buildah)
# 用法:
#   ./scripts/build_sandbox_buildah.sh --target cube --registry ccr.ccs.tencentyun.com/your-ns
#   ./scripts/build_sandbox_buildah.sh --target all --registry your-registry.example.com/weknora
# =============================================================================

set -euo pipefail

# 颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }

usage() {
    cat <<EOF
用法: $0 --target <target> --registry <registry> [选项]

必填:
  --target    构建目标: sandbox | cube | all
  --registry  推送的目标仓库地址 (如 ccr.ccs.tencentyun.com/my-ns)

可选:
  --tag       镜像标签 (默认: main)
  --no-push   仅构建，不推送
  --dry-run   打印将要执行的命令，不实际操作
  -h, --help  显示帮助

示例:
  # 构建 cube 并推送到腾讯云 CCR
  $0 --target cube --registry ccr.ccs.tencentyun.com/my-namespace

  # 构建所有沙箱镜像
  $0 --target all --registry harbor.internal.example.com/weknora

  # 仅本地构建，不推送
  $0 --target cube --registry ccr.ccs.tencentyun.com/my-ns --no-push

说明:
  此脚本使用 buildah 构建 OCI 镜像，完全不需要 Docker daemon。
  构建完成后推送到指定的私有仓库，Cube 集群从该仓库拉取镜像。
  之后在 WeKnora「设置 → 沙箱 → 编辑 → 模板」中
  填入完整镜像引用，如: ccr.ccs.tencentyun.com/my-ns/weknora-sandbox:main-cube
EOF
    exit 0
}

# 解析参数
TARGET=""
REGISTRY=""
TAG="main"
NO_PUSH=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --target)   TARGET="$2"; shift 2 ;;
        --registry) REGISTRY="$2"; shift 2 ;;
        --tag)      TAG="$2"; shift 2 ;;
        --no-push)  NO_PUSH=true; shift ;;
        --dry-run)  DRY_RUN=true; shift ;;
        -h|--help)  usage ;;
        *)          log_error "未知选项: $1"; usage ;;
    esac
done

if [[ -z "$TARGET" ]]; then
    log_error "缺少 --target"; usage
fi
if [[ "$NO_PUSH" != "true" && -z "$REGISTRY" ]]; then
    log_error "缺少 --registry（或使用 --no-push 跳过推送）"; usage
fi

# 规范化 registry：去掉尾部 /
REGISTRY="${REGISTRY%/}"

# 项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 镜像名前缀
IMAGE_PREFIX="${REGISTRY}/weknora-sandbox"

# 检查 buildah
check_buildah() {
    if ! command -v buildah &>/dev/null; then
        log_error "buildah 未安装。请运行: brew install buildah"
        exit 1
    fi
    log_info "buildah 版本: $(buildah --version)"
}

# 构建并推送单个 target
# 参数: $1 = Dockerfile target, $2 = 平台 (linux/amd64 或 linux/arm64), $3 = 标签后缀
build_and_push() {
    local target="$1"
    local platform="$2"
    local tag_suffix="$3"
    local full_tag="${IMAGE_PREFIX}:${TAG}${tag_suffix}"
    local dockerfile="${PROJECT_ROOT}/docker/Dockerfile.sandbox"

    log_info "构建 target=$target platform=$platform -> $full_tag"

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  buildah bud \\"
        echo "    --platform $platform \\"
        echo "    --build-arg TARGETPLATFORM=$platform \\"
        echo "    --build-arg TARGETARCH=${platform##*/} \\"
        echo "    --target $target \\"
        echo "    -t $full_tag \\"
        echo "    -f $dockerfile \\"
        echo "    $PROJECT_ROOT"
        if [[ "$NO_PUSH" != "true" ]]; then
            echo "  buildah push $full_tag"
        fi
        return 0
    fi

    # 构建
    buildah bud \
        --platform "$platform" \
        --build-arg "TARGETPLATFORM=$platform" \
        --build-arg "TARGETARCH=${platform##*/}" \
        --target "$target" \
        -t "$full_tag" \
        -f "$dockerfile" \
        "$PROJECT_ROOT"

    log_ok "构建完成: $full_tag"

    # 推送
    if [[ "$NO_PUSH" != "true" ]]; then
        log_info "推送 $full_tag ..."
        buildah push "$full_tag"
        log_ok "推送完成: $full_tag"
    fi
}

# 主流程
main() {
    check_buildah

    cd "$PROJECT_ROOT"

    # 确保 build 上下文中的文件存在
    if [[ ! -f "docker/Dockerfile.sandbox" ]]; then
        log_error "找不到 docker/Dockerfile.sandbox，请在项目根目录运行"
        exit 1
    fi
    case "$TARGET" in
        sandbox)
            # 基础 CLI 镜像，多架构
            local plat
            plat="$(uname -m)"
            [[ "$plat" == "arm64" ]] && plat="linux/arm64" || plat="linux/amd64"
            build_and_push "sandbox" "$plat" ""
            ;;
        cube)
            # Cube CLI 模板，仅 amd64
            build_and_push "cube" "linux/amd64" "-cube"
            ;;
        all)
            log_info "构建全部沙箱镜像..."

            # CLI 镜像: 使用本机架构
            local plat
            plat="$(uname -m)"
            [[ "$plat" == "arm64" ]] && plat="linux/arm64" || plat="linux/amd64"
            build_and_push "sandbox" "$plat" ""

            # Cube 变体: 固定 amd64（envd 来源 cubesandbox-base 只有 amd64）
            build_and_push "cube" "linux/amd64" "-cube"

            log_ok "全部镜像构建完成！"
            ;;
        *)
            log_error "未知 target: $TARGET"
            log_error "可选: sandbox | cube | all"
            exit 1
            ;;
    esac

    echo ""
    log_info "=== 下一步操作 ==="
    if [[ "$TARGET" == "cube" || "$TARGET" == "all" ]]; then
        echo ""
        echo "CLI Cube 模板镜像也已构建: ${IMAGE_PREFIX}:${TAG}-cube"
        echo "如需替换标准模板镜像，同样在沙箱编辑页面的模板设置中操作"
    fi
}

main