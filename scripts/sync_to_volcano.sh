#!/bin/bash
# =============================================================================
# WeKnora Sandbox 镜像同步脚本 — 服务器版 (Ubuntu + 火山云)
#
# 在服务器上使用 skopeo 将官方 sandbox 镜像从 Docker Hub 搬运到火山云容器镜像仓库。
# 无需 Docker daemon，无需构建，纯流式复制。
#
# 用法:
#   ./scripts/sync_to_volcano.sh \
#     --registry cr-cn-beijing.volces.com \
#     --namespace my-namespace \
#     --username your-username \
#     --password your-password
# =============================================================================

set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# =============================================================================
# 1. 参数
# =============================================================================
REGISTRY=""       # 火山云域名，如 cr-cn-beijing.volces.com
NAMESPACE=""      # 命名空间
USERNAME=""
PASSWORD=""
DRY_RUN=false

usage() {
    cat <<EOF
用法: $0 --registry <火山云域名> --namespace <命名空间> --username <用户名> --password <密码>

必填:
  --registry    火山云容器镜像域名 (如 cr-cn-beijing.volces.com)
  --namespace   命名空间
  --username    火山云镜像仓库用户名
  --password    火山云镜像仓库密码 / Token

可选:
  --dry-run     仅打印命令，不执行
  -h, --help    显示帮助

示例:
  $0 --registry cr-cn-beijing.volces.com \
     --namespace my-sandbox \
     --username user@example.com \
     --password "your-token-here"
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --registry)  REGISTRY="$2"; shift 2 ;;
        --namespace) NAMESPACE="$2"; shift 2 ;;
        --username)  USERNAME="$2"; shift 2 ;;
        --password)  PASSWORD="$2"; shift 2 ;;
        --dry-run)   DRY_RUN=true; shift ;;
        -h|--help)   usage ;;
        *)           log_error "未知选项: $1"; usage ;;
    esac
done

if [[ -z "$REGISTRY" || -z "$NAMESPACE" ]]; then
    log_error "缺少必填参数"
    usage
fi

REGISTRY="${REGISTRY%/}"
DEST_PREFIX="${REGISTRY}/${NAMESPACE}/weknora-sandbox"

# =============================================================================
# 2. 安装 skopeo
# =============================================================================
install_skopeo() {
    if command -v skopeo &>/dev/null; then
        log_ok "skopeo 已安装: $(skopeo --version 2>&1 | head -1)"
        return 0
    fi

    log_info "正在安装 skopeo ..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq skopeo

    if command -v skopeo &>/dev/null; then
        log_ok "skopeo 安装成功: $(skopeo --version 2>&1 | head -1)"
    else
        log_error "skopeo 安装失败，请手动安装: sudo apt-get install skopeo"
        exit 1
    fi
}

# =============================================================================
# 3. 登录
# =============================================================================
do_login() {
    local registry="$1" user="$2" pass="$3"
    if [[ -z "$user" || -z "$pass" ]]; then
        log_info "跳过 $registry 登录（未提供凭证）"
        return 0
    fi
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  skopeo login $registry -u $user -p ***"
        return 0
    fi
    log_info "登录 $registry ..."
    echo "$pass" | skopeo login "$registry" -u "$user" --password-stdin
    log_ok "已登录 $registry"
}

# =============================================================================
# 4. 检查源 + 复制
# =============================================================================
SOURCE="docker.io/wechatopenai/weknora-sandbox"

# 标准 Cube 模板镜像 tags
TAGS=(
    "main-cube"
    "latest-cube"
)

check_source() {
    local src="docker://${SOURCE}:$1"
    log_info "检查: $src"
    if skopeo inspect --no-tags "$src" &>/dev/null; then
        log_ok "源镜像存在"
        return 0
    else
        log_error "源镜像不存在或无法访问，跳过"
        return 1
    fi
}

copy_one() {
    local tag="$1"
    local src="docker://${SOURCE}:${tag}"
    local dst="docker://${DEST_PREFIX}:${tag}"

    log_info "复制 ${tag} ..."

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  skopeo copy --preserve-digests $src $dst"
        return 0
    fi

    if skopeo copy --preserve-digests "$src" "$dst"; then
        log_ok "完成: $dst"
    else
        log_error "失败: $tag（可能是网络问题，重试即可）"
    fi
}

# =============================================================================
# 5. 主流程
# =============================================================================
main() {
    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}  WeKnora Sandbox 镜像同步 → 火山云${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo "源:     $SOURCE"
    echo "目标:   $DEST_PREFIX"
    echo ""

    install_skopeo

    # 登录 Docker Hub（公开镜像一般不需要，但防止限流）
    do_login "docker.io" "${DOCKERHUB_USER:-}" "${DOCKERHUB_PASS:-}"

    # 登录火山云
    do_login "$REGISTRY" "$USERNAME" "$PASSWORD"

    echo ""

    local copied=0 failed=0
    for tag in "${TAGS[@]}"; do
        if check_source "$tag"; then
            copy_one "$tag" && ((copied++)) || ((failed++))
        else
            ((failed++))
        fi
    done

    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo "  结果: 成功 $copied / 失败 $failed"
    echo -e "${GREEN}================================================${NC}"

    if [[ "$copied" -gt 0 ]]; then
        echo ""
        log_info "标准 Cube 镜像已同步，请在集群构建模板后，在 WeKnora 中选择新模板。"
        echo "  ${DEST_PREFIX}:main-cube"

    fi
}

main