#!/usr/bin/env bash
# Ubuntu/Debian x86_64 KVM prerequisite installer and live API probe.
# Suitable as a preflight for CubeSandbox. Run --check-only for a read-only check.
set -Eeuo pipefail
export LC_ALL=C

CHECK_ONLY=0

usage() {
    cat <<'EOF'
用法
  sudo bash install_check_kvm.sh               安装工具、加载原生 KVM 驱动并检测
  sudo bash install_check_kvm.sh --check-only  只检测，不安装或加载驱动
  bash install_check_kvm.sh --help             查看帮助

安装模式支持 Ubuntu/Debian x86_64，安装 qemu-kvm、cpu-checker 和 python3。
检测内容：CPU 虚拟化标志、模块签名限制、/dev/kvm、KVM API 和创建空 VM。
保留现有内核、GRUB 与签名策略；已有 PVM 接口可以检测。

退出码
  0  KVM API 正常且空 VM 创建成功
  1  参数、系统、依赖或安装错误
  2  KVM 当前不可用，或当前账号无设备访问权限
EOF
}

info() { printf '[INFO] %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*" >&2; }
fail() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }

while (($#)); do
    case "$1" in
        --check-only) CHECK_ONLY=1 ;;
        -h|--help) usage; exit 0 ;;
        *) usage >&2; fail "未知参数：$1" ;;
    esac
    shift
done

[[ $(uname -s) == Linux ]] || fail '请在 Linux 服务器上运行。'
[[ $(uname -m) == x86_64 ]] || fail '此脚本适用于 x86_64 服务器。'

if (( ! CHECK_ONLY )); then
    [[ $EUID -eq 0 ]] || fail '安装模式需要 root，请使用 sudo bash 运行。'
    [[ -r /etc/os-release ]] || fail '无法识别 Linux 发行版。'
    # This root-owned OS file is the distribution's standard identification API.
    . /etc/os-release
    case "${ID:-}" in
        ubuntu|debian) ;;
        *) fail '安装模式仅支持 Ubuntu/Debian；其他系统可使用 --check-only。' ;;
    esac
    command -v apt-get >/dev/null || fail '未找到 apt-get。'
    info '从当前配置的软件源安装 KVM 工具。'
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y qemu-kvm cpu-checker python3
fi

command -v python3 >/dev/null || fail '检测需要 python3；请安装后再执行。'
info "当前内核：$(uname -r)"
[[ -r /proc/cpuinfo ]] || fail '无法读取 /proc/cpuinfo。'
[[ -r /proc/modules ]] || fail '无法读取 /proc/modules。'

CPU_BACKEND=$(awk '
    /^flags[[:space:]]*:/ {
        for (i = 3; i <= NF; i++) {
            if ($i == "vmx") { print "kvm_intel"; exit }
            if ($i == "svm") { print "kvm_amd"; exit }
        }
    }
' /proc/cpuinfo)

PVM_ACTIVE=0
if awk '$1 == "kvm_pvm" { found=1 } END { exit !found }' /proc/modules; then
    PVM_ACTIVE=1
fi

SIG_ENFORCED=0
if [[ -r /sys/module/module/parameters/sig_enforce ]]; then
    sig_value=$(cat /sys/module/module/parameters/sig_enforce)
    case "$sig_value" in Y|y|1) SIG_ENFORCED=1 ;; esac
elif [[ -r /proc/cmdline ]] && grep -Eq '(^|[[:space:]])module\.sig_enforce=1([[:space:]]|$)' /proc/cmdline; then
    SIG_ENFORCED=1
fi
if (( SIG_ENFORCED )); then
    info '内核模块签名强制：已开启，将保留。'
else
    info '未检测到 module.sig_enforce 强制开启；脚本保持现有策略。'
fi

if [[ -n "$CPU_BACKEND" ]]; then
    info "CPU 已暴露硬件虚拟化标志，对应驱动：$CPU_BACKEND"
else
    warn 'CPU 未暴露 vmx/svm：云服务器可能没有开放传统嵌套虚拟化。'
fi
if (( PVM_ACTIVE )); then
    info '当前已加载 kvm_pvm，将检测现有 PVM 提供的 KVM 接口。'
fi

# Never try to load an unsigned module when signature enforcement is active.
# Never load PVM automatically; the user may deliberately have deferred it.
module_signature_ok() {
    local module=$1 signer
    if (( ! SIG_ENFORCED )); then return 0; fi
    command -v modinfo >/dev/null || {
        warn '未找到 modinfo，无法核对驱动签名。'
        return 1
    }
    if ! signer=$(modinfo -F signer "$module" 2>/dev/null); then
        warn "当前内核没有可查询的 $module 驱动。"
        return 1
    fi
    if [[ -z "$signer" ]]; then
        warn "$module 模块没有签名，当前签名策略会拒绝加载。"
        return 1
    fi
    info "$module 签名者：$signer"
}

if (( ! CHECK_ONLY && ! PVM_ACTIVE )) && [[ ! -e /dev/kvm && -n "$CPU_BACKEND" ]]; then
    command -v modprobe >/dev/null || fail '未找到 modprobe。'
    if module_signature_ok kvm && module_signature_ok "$CPU_BACKEND"; then
        info "加载原生驱动 $CPU_BACKEND。"
        if ! modprobe "$CPU_BACKEND"; then
            warn '驱动加载失败。检查 BIOS/云厂商嵌套虚拟化设置及内核日志。'
        fi
    else
        warn '保持现有签名策略，跳过不满足签名条件的驱动加载。'
    fi
fi

if [[ ! -e /dev/kvm ]]; then
    warn '/dev/kvm 不存在，KVM 当前不可用。'
    if [[ -z "$CPU_BACKEND" ]]; then
        warn '安装 QEMU/KVM 工具不能让云厂商未透传的 VMX/SVM 自动出现。'
        warn '请选用支持嵌套虚拟化的实例，或另行准备符合签名策略的 PVM 环境。'
    elif (( CHECK_ONLY )); then
        warn "CPU 标志存在，但驱动可能未加载；可用安装模式加载 $CPU_BACKEND 后检查。"
    fi
    if [[ $(uname -r) == *cubesandbox.pvm.host* ]] && command -v modinfo >/dev/null; then
        # Diagnose an installed PVM kernel without changing the user's choice.
        if (( SIG_ENFORCED )) && ! module_signature_ok kvm_pvm; then
            warn 'PVM 驱动与现有签名要求不兼容；脚本不会修改 GRUB 或关闭校验。'
        fi
    fi
    exit 2
fi

[[ -c /dev/kvm ]] || { warn '/dev/kvm 不是字符设备。'; exit 2; }
ls -l /dev/kvm
if [[ ! -r /dev/kvm || ! -w /dev/kvm ]]; then
    warn '当前账号没有 /dev/kvm 读写权限。请用 sudo 检测，或按运维策略配置 kvm 组权限。'
    exit 2
fi

info '通过真实内核 API 创建空 VM，再立即释放资源。'
if python3 - <<'PY'
import fcntl
import os
import sys

kvm_fd = vm_fd = None
try:
    kvm_fd = os.open('/dev/kvm', os.O_RDWR | os.O_CLOEXEC)
    version = fcntl.ioctl(kvm_fd, 0xAE00, 0)  # KVM_GET_API_VERSION
    print(f'KVM_API_VERSION={version}')
    if version != 12:
        raise RuntimeError(f'不支持的 KVM API 版本 {version}，预期为 12')
    vm_fd = fcntl.ioctl(kvm_fd, 0xAE01, 0)  # KVM_CREATE_VM, default machine type
    print('KVM_CREATE_VM=成功')
except (OSError, RuntimeError) as exc:
    print(f'[ERROR] KVM API 检测失败：{exc}', file=sys.stderr)
    sys.exit(2)
finally:
    if vm_fd is not None:
        os.close(vm_fd)
    if kvm_fd is not None:
        os.close(kvm_fd)
PY
then
    info '检测通过：KVM 接口可访问，空虚拟机创建成功。'
    if (( PVM_ACTIVE )); then
        info '这是 PVM 提供的接口，不代表云厂商开放了硬件嵌套虚拟化。'
    fi
    info '下一步可验证 CubeSandbox 模板和 guest 启动；本检测不等于业务沙箱验收。'
else
    exit 2
fi
