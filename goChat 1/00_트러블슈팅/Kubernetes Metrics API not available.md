# Kubernetes Metrics API not available

## 문제

Pod의 CPU와 메모리를 확인하기 위해 다음 명령을 실행하였다.

```bash
kubectl top pods -n chat
```

오류:

```text
error: Metrics API not available
```

---

## 원인

`kubectl top`은 Kubernetes에 기본 저장된 Pod 목록만 읽는 명령이 아니다. Metrics API를 제공하는 Metrics Server가 필요하다.

```text
kubectl top
→ Metrics API 요청
→ Metrics Server 필요
```

Minikube Cluster에 Metrics Server가 아직 설치되지 않아 API를 사용할 수 없었다.

---

## 해결

Minikube Addon으로 Metrics Server를 설치하였다.

```bash
minikube addons enable metrics-server
```

설치 완료 대기:

```bash
kubectl rollout status \
  deployment/metrics-server \
  -n kube-system \
  --timeout=120s
```

---

## 결과

```bash
kubectl top nodes
kubectl top pods -n chat
```

Node와 Chat Pod, PostgreSQL, Redis의 CPU 및 메모리 사용량이 정상적으로 출력되었다.

---

## Metrics Server와 Prometheus 차이

Metrics Server는 현재 CPU·메모리 조회와 HPA의 기본 판단 자료를 제공한다.

Prometheus는 여러 시점의 Metric을 장기간 수집하고 저장하여 추세, 장애 시점, 처리량 등을 분석한다.

따라서 Metrics Server를 설치했다고 Prometheus와 Grafana가 불필요해지는 것은 아니다.
