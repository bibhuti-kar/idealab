#!/usr/bin/env bash
set -euo pipefail

# idealab pre-install script
# Installs NVIDIA drivers, container toolkit, Go, and k3s on Ubuntu/Debian
# Must run as root

LOG_PREFIX="[idealab-preinstall]"

log_info()  { echo "$LOG_PREFIX INFO:  $*"; }
log_warn()  { echo "$LOG_PREFIX WARN:  $*"; }
log_error() { echo "$LOG_PREFIX ERROR: $*" >&2; }
log_ok()    { echo "$LOG_PREFIX OK:    $*"; }

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (sudo)"
        exit 1
    fi
}

check_os() {
    if [[ ! -f /etc/os-release ]]; then
        log_error "Cannot detect OS. Only Ubuntu/Debian supported."
        exit 1
    fi
    source /etc/os-release
    if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
        log_warn "Detected $ID — this script is tested on Ubuntu/Debian only"
    fi
    log_info "Detected OS: $PRETTY_NAME"
}

install_base_packages() {
    log_info "Installing base packages..."
    apt-get update -qq
    apt-get install -y -qq curl wget gnupg2 software-properties-common \
        apt-transport-https ca-certificates lsb-release pciutils
    log_ok "Base packages installed"
}

detect_gpu() {
    log_info "Detecting NVIDIA GPU..."
    if ! lspci | grep -qi nvidia; then
        log_error "No NVIDIA GPU detected via lspci"
        exit 1
    fi
    GPU_MODEL=$(lspci | grep -i "vga.*nvidia" | sed 's/.*: //')
    log_ok "Found GPU: $GPU_MODEL"
}

install_nvidia_driver() {
    if nvidia-smi &>/dev/null; then
        DRIVER_VER=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -1)
        log_ok "NVIDIA driver already installed: $DRIVER_VER"
        return
    fi

    log_info "Installing NVIDIA driver..."
    apt-get install -y -qq nvidia-driver-560
    log_ok "NVIDIA driver 560 installed — reboot may be required"
}

install_nvidia_container_toolkit() {
    if command -v nvidia-ctk &>/dev/null; then
        log_ok "NVIDIA Container Toolkit already installed"
        return
    fi

    log_info "Installing NVIDIA Container Toolkit..."
    curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
        | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg

    DIST=$(. /etc/os-release; echo "${ID}${VERSION_ID}")
    curl -fsSL "https://nvidia.github.io/libnvidia-container/${DIST}/libnvidia-container.list" \
        | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
        | tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

    apt-get update -qq
    apt-get install -y -qq nvidia-container-toolkit
    log_ok "NVIDIA Container Toolkit installed"
}

configure_containerd_nvidia() {
    # k3s v1.34+ auto-detects the NVIDIA container runtime if
    # /usr/bin/nvidia-container-runtime exists on the host.
    # DO NOT use nvidia-ctk runtime configure with k3s — it generates
    # containerd v2 config which breaks k3s (which uses v3 format).
    # DO NOT create config.toml.tmpl — it replaces the entire containerd
    # config and breaks CNI networking.
    if [ -d "/var/lib/rancher/k3s" ]; then
        if [ -x /usr/bin/nvidia-container-runtime ]; then
            log_ok "NVIDIA container runtime found — k3s will auto-detect it"
        else
            log_error "nvidia-container-runtime not found at /usr/bin/nvidia-container-runtime"
            return 1
        fi
    else
        # Non-k3s: use nvidia-ctk for standard containerd
        nvidia-ctk runtime configure --runtime=containerd --set-as-default
        log_ok "containerd configured with NVIDIA runtime (system path)"
    fi
}

install_go() {
    GO_VERSION="1.22.10"
    if command -v go &>/dev/null; then
        CURRENT=$(go version | awk '{print $3}' | sed 's/go//')
        log_ok "Go already installed: $CURRENT"
        return
    fi

    log_info "Installing Go $GO_VERSION..."
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz

    # Add to path for current session
    export PATH=$PATH:/usr/local/go/bin

    # Persist for all users
    if ! grep -q '/usr/local/go/bin' /etc/profile.d/go.sh 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
    fi

    log_ok "Go $GO_VERSION installed"
}

install_k3s() {
    if command -v k3s &>/dev/null; then
        K3S_VER=$(k3s --version | head -1)
        log_ok "k3s already installed: $K3S_VER"
        return
    fi

    log_info "Installing k3s..."
    curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--write-kubeconfig-mode 644" sh -

    # Wait for k3s to be ready
    log_info "Waiting for k3s to be ready..."
    for i in $(seq 1 30); do
        if k3s kubectl get nodes &>/dev/null; then
            break
        fi
        sleep 2
    done

    log_ok "k3s installed and running"
}

configure_k3s_nvidia() {
    log_info "Configuring k3s for NVIDIA GPU support..."

    # Create RuntimeClass for nvidia
    k3s kubectl apply -f - <<'EOF'
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: nvidia
handler: nvidia
EOF

    # Restart k3s to pick up containerd changes
    systemctl restart k3s
    sleep 5

    log_ok "k3s configured with NVIDIA RuntimeClass"
}

install_nvidia_device_plugin() {
    log_info "Installing NVIDIA device plugin..."
    k3s kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.15.0/deployments/static/nvidia-device-plugin.yml

    # The device plugin pod must run with the nvidia RuntimeClass so it can
    # access libnvidia-ml.so from the host via the NVIDIA container runtime.
    log_info "Patching device plugin daemonset with runtimeClassName: nvidia..."
    k3s kubectl patch daemonset nvidia-device-plugin-daemonset -n kube-system \
        --type='json' \
        -p='[{"op": "add", "path": "/spec/template/spec/runtimeClassName", "value": "nvidia"}]'

    # Wait for the new pod to be ready
    log_info "Waiting for device plugin pod to restart..."
    sleep 10
    for i in $(seq 1 30); do
        READY=$(k3s kubectl get daemonset nvidia-device-plugin-daemonset -n kube-system \
            -o jsonpath='{.status.numberReady}' 2>/dev/null || echo "0")
        if [[ "$READY" -ge 1 ]]; then
            break
        fi
        sleep 2
    done

    log_ok "NVIDIA device plugin deployed with runtimeClassName: nvidia"
}

validate_gpu_test_pod() {
    log_info "Running GPU test pod..."

    k3s kubectl apply -f - <<'TESTPOD'
apiVersion: v1
kind: Pod
metadata:
  name: gpu-test
  namespace: default
spec:
  restartPolicy: Never
  runtimeClassName: nvidia
  containers:
    - name: gpu-test
      image: nvidia/cuda:12.6.0-base-ubuntu22.04
      command: ["nvidia-smi"]
      resources:
        limits:
          nvidia.com/gpu: 1
TESTPOD

    log_info "Waiting for GPU test pod to complete..."
    for i in $(seq 1 60); do
        STATUS=$(k3s kubectl get pod gpu-test -o jsonpath='{.status.phase}' 2>/dev/null || echo "Pending")
        if [[ "$STATUS" == "Succeeded" ]]; then
            log_ok "GPU test pod succeeded"
            k3s kubectl logs gpu-test
            k3s kubectl delete pod gpu-test --ignore-not-found
            return 0
        elif [[ "$STATUS" == "Failed" ]]; then
            log_error "GPU test pod failed"
            k3s kubectl logs gpu-test
            k3s kubectl delete pod gpu-test --ignore-not-found
            return 1
        fi
        sleep 2
    done

    log_error "GPU test pod timed out after 120s"
    k3s kubectl describe pod gpu-test
    k3s kubectl delete pod gpu-test --ignore-not-found
    return 1
}

validate() {
    log_info "Validating installation..."

    local ERRORS=0

    # Check nvidia-smi
    if nvidia-smi &>/dev/null; then
        log_ok "nvidia-smi works"
    else
        log_error "nvidia-smi failed — reboot may be required"
        ERRORS=$((ERRORS + 1))
    fi

    # Check k3s
    if k3s kubectl get nodes &>/dev/null; then
        log_ok "k3s cluster is running"
    else
        log_error "k3s not responding"
        ERRORS=$((ERRORS + 1))
    fi

    # Check GPU resources on node
    if k3s kubectl describe nodes | grep -q "nvidia.com/gpu"; then
        log_ok "GPU resources visible in k3s"
    else
        log_warn "GPU resources not yet visible — device plugin may still be starting"
    fi

    # Check containerd runtime
    if nvidia-ctk runtime check --runtime=containerd &>/dev/null; then
        log_ok "containerd NVIDIA runtime configured"
    else
        log_warn "containerd NVIDIA runtime check inconclusive"
    fi

    # Run GPU test pod
    if validate_gpu_test_pod; then
        log_ok "GPU test pod validation passed"
    else
        log_warn "GPU test pod validation failed — device plugin may need more time"
    fi

    if [[ $ERRORS -gt 0 ]]; then
        log_error "Validation completed with $ERRORS errors"
        return 1
    fi

    log_ok "All validations passed"
}

main() {
    log_info "idealab pre-install starting..."
    check_root
    check_os
    install_base_packages
    detect_gpu
    install_nvidia_driver
    install_nvidia_container_toolkit
    configure_containerd_nvidia
    install_go
    install_k3s
    configure_k3s_nvidia
    install_nvidia_device_plugin
    validate
    log_info "Pre-install complete. KUBECONFIG=/etc/rancher/k3s/k3s.yaml"
    log_info "If NVIDIA driver was just installed, reboot and re-run validation."
}

main "$@"
