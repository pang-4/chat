## 문제

`kube-prometheus-stack`으로 Prometheus와 Grafana를 설치한 뒤 Grafana의 다음 기본 대시보드를 확인하였다.

```text
Kubernetes / Compute Resources / Namespace (Pods)
```

`chat` Namespace를 선택하면 Pod 목록과 CPU Request, CPU Limit 같은 선언 값은 표시되었지만 다음 사용량 패널에는 `No data`가 표시되었다.

```text
CPU Utilisation
Memory Utilisation
CPU Usage
Memory Usage
Receive Bandwidth
Transmit Bandwidth
```

한편 채팅 서버에서 직접 제공하는 WebSocket 연결 수 Metric은 정상적으로 조회되었다.

```promql
sum(chat_websocket_connections)
```

따라서 Grafana 전체 또는 Prometheus 전체의 연결 문제가 아니라 Kubernetes Container Metric 수집 경로의 문제일 가능성이 있었다.

---

## Metric 수집 경로 구분

채팅 서버의 WebSocket Metric과 Kubernetes의 CPU, Memory Metric은 서로 다른 경로로 수집된다.

```text
WebSocket 연결 수
chat-app /metrics
→ ServiceMonitor
→ Prometheus
→ Grafana

Pod CPU·Memory·Network
kubelet/cAdvisor
→ kubelet ServiceMonitor
→ Prometheus
→ Grafana
```

따라서 `chat_websocket_connections`가 정상이라고 해서 kubelet/cAdvisor Metric까지 정상이라는 의미는 아니다.

---

## 애플리케이션 Metric 확인

Chat Service를 로컬 8080 포트로 전달하였다.

```bash
kubectl port-forward \
  service/chat-app \
  8080:8080 \
  -n chat
```

다른 Terminal에서 `/metrics`를 조회하였다.

```bash
curl http://localhost:8080/metrics
```

다음 Metric이 정상적으로 출력되었다.

```text
# HELP chat_websocket_connections Current number of active WebSocket connections.
# TYPE chat_websocket_connections gauge
chat_websocket_connections 0
```

Prometheus API에서도 같은 Metric을 조회하였다.

```bash
kubectl port-forward \
  service/monitoring-kube-prometheus-prometheus \
  9090:9090 \
  -n monitoring
```

```bash
curl -s \
  'http://localhost:9090/api/v1/query?query=chat_websocket_connections'
```

Chat Pod 3개의 결과가 반환되었다. 애플리케이션 Metric 수집 경로는 정상이었다.

전체 WebSocket 연결 수는 Grafana에서 다음과 같이 조회할 수 있다.

```promql
sum(chat_websocket_connections)
```

---

## Kubernetes Container Metric 확인

Grafana Explore에서 다음 PromQL을 실행하였다.

```promql
count(container_cpu_usage_seconds_total{
  namespace="chat",
  container!=""
})
```

```promql
count(container_memory_working_set_bytes{
  namespace="chat",
  container!=""
})
```

두 Query 모두 `No data`를 반환하였다.

Grafana 기본 Kubernetes Dashboard는 실제 Container를 선택하기 위해 일반적으로 다음 조건을 사용한다.

```promql
container!=""
```

따라서 Prometheus에 `container` Label이 있는 시계열이 없으면 기본 Dashboard가 데이터를 표시할 수 없다.

---

## kubelet ServiceMonitor와 Target 확인

kubelet ServiceMonitor가 존재하는지 확인하였다.

```bash
kubectl get servicemonitor -A \
  | grep -E 'kubelet|cadvisor'
```

```text
monitoring   monitoring-kube-prometheus-kubelet
```

ServiceMonitor가 cAdvisor Endpoint를 포함하는지도 확인하였다.

```bash
kubectl get servicemonitor \
  monitoring-kube-prometheus-kubelet \
  -n monitoring \
  -o jsonpath='{range .spec.endpoints[*]}{.port}{" path="}{.path}{"\n"}{end}'
```

```text
https-metrics path=
https-metrics path=/metrics/cadvisor
https-metrics path=/metrics/probes
```

Prometheus의 kubelet Target 상태를 확인하였다.

```bash
curl -sG \
  --data-urlencode 'query=up{job="kubelet"}' \
  http://localhost:9090/api/v1/query
```

`/metrics`, `/metrics/cadvisor`, `/metrics/probes` Target이 모두 `up=1`이었다.

Prometheus가 cAdvisor Endpoint에서 받은 Sample 수도 확인하였다.

```bash
curl -sG \
  --data-urlencode 'query=scrape_samples_scraped{job="kubelet",metrics_path="/metrics/cadvisor"}' \
  http://localhost:9090/api/v1/query
```

수천 개의 Sample이 반환되었다. ServiceMonitor, Network, 인증, Scrape 자체는 정상이었다.

---

## cAdvisor 원본 Metric 확인

Kubernetes API Server Proxy를 통해 cAdvisor 원본 Metric을 조회하였다.

```bash
kubectl get --raw \
  /api/v1/nodes/minikube/proxy/metrics/cadvisor \
  | grep '^container_cpu_usage_seconds_total' \
  | grep 'namespace="chat"' \
  | head
```

CPU Metric은 존재했지만 다음 Label이 비어 있었다.

```text
container=""
image=""
name=""
```

실제 Container 이름이 있는 Metric만 검색하면 아무 결과도 나오지 않았다.

```bash
kubectl get --raw \
  /api/v1/nodes/minikube/proxy/metrics/cadvisor \
  | grep '^container_cpu_usage_seconds_total' \
  | grep 'namespace="chat"' \
  | grep -v 'container=""' \
  | head
```

즉 cAdvisor는 Pod cgroup의 집계 값은 수집했지만 Docker Container를 식별하지 못했다.

---

## 환경 확인

Kubernetes와 Minikube 환경을 확인하였다.

```bash
kubectl version
minikube version

kubectl get node minikube \
  -o jsonpath='Kubelet: {.status.nodeInfo.kubeletVersion}{"\n"}Runtime: {.status.nodeInfo.containerRuntimeVersion}{"\n"}OS: {.status.nodeInfo.osImage}{"\n"}Kernel: {.status.nodeInfo.kernelVersion}{"\n"}'
```

```text
Kubernetes Server: v1.35.1
Minikube: v1.38.1
Runtime: docker://29.2.1
OS: Debian GNU/Linux 12
Kernel: 6.8.0-136-generic
```

Minikube Profile도 확인하였다.

```bash
minikube profile list
```

```text
DRIVER    docker
RUNTIME   docker
VERSION   v1.35.1
```

여기서 Driver와 Runtime은 의미가 다르다.

```text
Driver docker
→ Minikube Node 자체를 Docker Container로 실행

Runtime docker
→ Kubernetes Pod도 Minikube 내부 Docker로 실행
```

호스트 Docker가 아니라 Minikube 내부 Docker 버전과 API를 확인하였다.

```bash
minikube ssh -- \
  'docker version --format "Docker={{.Server.Version}} API={{.Server.APIVersion}} MIN={{.Server.MinAPIVersion}}"'
```

```text
Docker=29.2.1 API=1.53 MIN=1.44
```

---

## 원인

Minikube 내부 Docker 29.2.1은 Docker API 최소 버전으로 `1.44`를 요구하였다.

Kubernetes v1.35.1에 내장된 cAdvisor의 Docker Client가 이 API 조합에서 Docker Factory를 정상 등록하지 못하여 Container Metadata를 가져오지 못했다.

그 결과 cAdvisor에는 raw cgroup 기반의 Pod 집계 Metric만 남았다.

```text
CPU·Memory 값 존재
        +
container="", image="", name=""
        ↓
Grafana 기본 Query의 container!="" 조건에서 제외
        ↓
No data
```

Prometheus Target은 `up=1`이고 Scrape Sample도 존재했기 때문에 단순 연결 실패로 보이지 않는 점에 주의해야 한다.

Docker 29와 cAdvisor의 호환성 문제는 다음 Issue에서도 확인할 수 있다.

- [Failure to get data in Prometheus on latest Docker](https://github.com/google/cadvisor/issues/3749)
- [cAdvisor fails to collect container metrics on Docker 29.x](https://github.com/google/cadvisor/issues/3860)
- [Docker Engine version 29 release notes](https://docs.docker.com/engine/release-notes/29/)

---

## 임시 확인용 Query

문제를 진단하는 동안 `container` Label을 요구하지 않는 다음 Query로 Pod cgroup 집계 값이 존재하는지 확인하였다.

```promql
sum(rate(container_cpu_usage_seconds_total{
  namespace="chat",
  pod!=""
}[5m]))
```

```promql
sum(container_memory_working_set_bytes{
  namespace="chat",
  pod!=""
})
```

값이 출력되어 Metric 자체는 존재한다는 사실을 확인할 수 있었다.

이 Query는 진단용 우회 방법이다. Container Label이 복구된 뒤에도 사용하면 Pod 집계와 Container 값을 함께 합산하여 중복 계산할 수 있으므로 장기 해결책으로 사용하지 않는다.

---

## 해결

Minikube Node를 실행하는 Driver는 Docker로 유지하고 Kubernetes의 Container Runtime만 `containerd`로 변경하였다.

```bash
minikube start \
  --driver=docker \
  --container-runtime=containerd
```

실행 결과:

```text
Preparing Kubernetes v1.35.1 on containerd 2.2.1
Done! kubectl is now configured to use "minikube" cluster
```

이번 환경에서는 기존 Profile이 갱신되었으며 Namespace, Pod, PVC도 유지되었다.

하지만 Runtime 변경은 Cluster 상태에 영향을 줄 수 있다. 다른 환경에서 같은 결과를 보장할 수 없으므로 실행 전에 다음 데이터를 백업한다.

- PostgreSQL Data
- Kubernetes Secret
- ArgoCD Repository 인증 정보
- Grafana에서 직접 만든 Dashboard JSON
- Prometheus Data가 필요한 경우 Prometheus Volume

`minikube delete`는 Cluster와 Volume을 삭제할 수 있으므로 백업 없이 실행하지 않는다.

---

## Runtime 변경 결과 확인

Minikube Runtime이 `containerd`로 변경되었는지 확인하였다.

```bash
minikube profile list
```

```text
DRIVER    docker
RUNTIME   containerd
VERSION   v1.35.1
STATUS    OK
```

기존 Kubernetes Resource와 PVC가 유지되었는지 확인하였다.

```bash
kubectl get namespaces
kubectl get pods -A
kubectl get pvc -A
```

다음 Namespace와 주요 Workload가 모두 정상 상태였다.

```text
argocd
chat
monitoring
```

```text
chat-app       Running
postgres       Running
redis          Running
prometheus     Running
grafana        Running
argocd         Running
```

PostgreSQL PVC도 `Bound` 상태를 유지하였다.

---

## Container Label 복구 확인

cAdvisor 원본 Metric을 다시 조회하였다.

```bash
kubectl get --raw \
  /api/v1/nodes/minikube/proxy/metrics/cadvisor \
  | grep '^container_cpu_usage_seconds_total' \
  | grep 'namespace="chat"' \
  | grep -v 'container=""' \
  | head
```

Runtime 변경 후 다음 Label이 정상적으로 표시되었다.

```text
container="chat-app"
image="ghcr.io/pang-4/chat-app:<COMMIT_SHA>"
name="<CONTAINER_ID>"
namespace="chat"
pod="chat-app-..."
```

Prometheus가 새로운 Label을 수집하도록 잠시 기다린 뒤 Grafana 기본 Dashboard를 새로 고쳤다.

```text
Kubernetes / Compute Resources / Namespace (Pods)
```

다음 패널에 값이 정상적으로 표시되었다.

- CPU Utilisation from requests
- CPU Utilisation from limits
- Memory Utilisation from requests
- Memory Utilisation from limits
- Pod별 CPU Usage
- Pod별 Memory Usage
- Network Receive·Transmit

Runtime 변경 이후부터 정상 Label의 시계열이 생성되므로 그래프의 이전 구간은 비어 있을 수 있다.

---

## ServiceMonitor와 Runtime 문제의 차이

Metric이 보이지 않을 때는 먼저 문제가 발생한 단계를 구분한다.

### 애플리케이션 `/metrics`도 조회되지 않음

다음을 점검한다.

- 애플리케이션 `/metrics` Handler
- Kubernetes Service Port
- Service Label
- ServiceMonitor Selector
- Prometheus의 ServiceMonitor Selector

### 애플리케이션 Metric은 보이지만 Kubernetes 기본 Dashboard가 `No data`

다음을 점검한다.

- kubelet ServiceMonitor
- Prometheus의 kubelet Target `up`
- `/metrics/cadvisor` Sample 수
- `container`, `image`, `name`, `namespace`, `pod` Label
- Kubernetes와 Container Runtime, cAdvisor의 버전 호환성

---

## 재발 방지

Minikube Cluster를 시작할 때 Runtime을 명시한다.

```bash
minikube start \
  --driver=docker \
  --container-runtime=containerd
```

상태 확인 시 Driver뿐 아니라 Runtime도 함께 확인한다.

```bash
minikube profile list
```

모니터링 설치 후에는 Grafana 화면만 확인하지 않고 Prometheus와 cAdvisor를 단계별로 검증한다.

```bash
# kubelet Target
curl -sG \
  --data-urlencode 'query=up{job="kubelet"}' \
  http://localhost:9090/api/v1/query

# Container Label
kubectl get --raw \
  /api/v1/nodes/minikube/proxy/metrics/cadvisor \
  | grep '^container_cpu_usage_seconds_total' \
  | grep -v 'container=""' \
  | head
```

Kubernetes, Minikube, Docker를 업그레이드할 때는 다음 호환성을 함께 확인한다.

- Kubernetes와 Minikube
- kubelet 내장 cAdvisor와 Container Runtime
- Docker Engine API 최소 버전
- kube-prometheus-stack과 Kubernetes

---

##  배운ㅁ
- Grafana의 No data는 반드시 Grafana 연결 실패를 의미하지 않는다.
- 애플리케이션 Metric과 Kubernetes 자원 Metric은 서로 다른 경로로 수집된다.
- Prometheus Target이 `up=1`이어도 필요한 Label이 없으면 기본 Dashboard는 데이터를 표시하지 못한다.
- cAdvisor 원본 Metric을 확인하면 수집 실패와 Label 누락을 구분할 수 있다.
- Minikube의 Driver와 Kubernetes Container Runtime은 서로 다른 설정이다.
- Docker Driver를 사용하면서 Pod Runtime은 containerd로 구성할 수 있다.
- 임시 PromQL 우회보다 Metric Pipeline과 Label을 정상화하는 것이 장기적으로 안전하다.
