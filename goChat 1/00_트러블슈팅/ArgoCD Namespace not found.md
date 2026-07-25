# ArgoCD Namespace not found

## 문제

ArgoCD 설치 상태를 확인하려고 다음 명령을 실행하였다.

```bash
kubectl get pods -n argocd -w
```

오류가 반복 출력되었다.

```text
Error from server (NotFound): namespaces "argocd" not found
```

---

## 원인

`kubectl get pods -n argocd`는 ArgoCD를 설치하는 명령이 아니라, 이미 존재하는 `argocd` Namespace의 Pod를 조회하는 명령이다.

Namespace 생성과 ArgoCD 설치보다 조회 명령을 먼저 실행하였다.

`-w`를 사용하여 같은 조회를 계속 반복했기 때문에 오류도 반복해서 출력되었다.

---

## 해결

Watch를 `Ctrl+C`로 종료하였다.

ArgoCD Namespace 생성:

```bash
kubectl create namespace argocd
```

ArgoCD 설치:

```bash
kubectl apply \
  -n argocd \
  --server-side \
  --force-conflicts \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.4.2/manifests/install.yaml
```

그다음 Pod를 관찰하였다.

```bash
kubectl get pods -n argocd -w
```

---

## 결과

ArgoCD의 Server, Controller, Repo Server, Redis 등의 Pod가 순차적으로 `1/1 Running` 상태가 되었다.

---

## 배운 점

```text
create/apply = 자원 생성·변경
get          = 현재 자원 조회
get -w       = 자원 상태 변화를 계속 관찰
```

조회 명령은 존재하지 않는 Namespace나 자원을 자동으로 생성하지 않는다.
