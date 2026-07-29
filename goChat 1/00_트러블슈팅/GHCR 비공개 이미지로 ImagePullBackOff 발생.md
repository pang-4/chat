## 문제

Chat Deployment의 이미지를 Minikube 로컬 이미지에서 GHCR 이미지로 변경하였다.

```yaml
image: ghcr.io/pang-4/chat-app:8dd58c79cb5bda5bb63bc833a597f4612b8b64aa
```

적용 후 Rollout이 다음 상태에서 진행되지 않았다.

```text
Waiting for deployment "chat-app" rollout to finish:
1 out of 3 new replicas have been updated...
```

Pod 상태:

```bash
kubectl get pods -n chat -l app=chat-app
```

```text
새 Pod    0/1   ImagePullBackOff
기존 Pod  1/1   Running
기존 Pod  1/1   Running
기존 Pod  1/1   Running
```

---

## 의미

`ImagePullBackOff`는 Kubernetes가 컨테이너 이미지를 Registry에서 내려받지 못했고, 재시도 간격을 늘리며 기다리는 상태다.

애플리케이션 실행 전의 문제이므로 Go 로그보다 Pod Events를 먼저 확인해야 한다.

```bash
kubectl describe pod <NEW_CHAT_POD> -n chat
```

---

## 원인

GitHub Actions는 GHCR 이미지 게시에 성공했지만 새 Package의 Visibility가 `Private`이었다.

```text
Kubernetes
→ ghcr.io/pang-4/chat-app Pull 요청
→ 인증정보 없음
→ Private Package 접근 거부
→ ImagePullBackOff
```

GitHub Repository가 공개되어 있어도 GHCR Package Visibility는 별도로 확인해야 한다.

---

## 해결

GitHub에서 다음 메뉴로 이동하였다.

```text
Profile
→ Packages
→ chat-app
→ Package settings
→ Danger Zone
→ Change visibility
→ Public
```

공개 Package는 인증 없이 Pull할 수 있다. 공개 전환은 되돌릴 수 없으므로 공개 포트폴리오 이미지인지 확인한 뒤 변경한다.

실패한 Pod를 삭제하여 즉시 다시 생성하게 하였다.

```bash
kubectl delete pod <IMAGE_PULL_BACKOFF_POD> -n chat
```

상태 관찰:

```bash
kubectl get pods -n chat -l app=chat-app -w
```

---

## 결과

새 Pod 세 개가 GHCR 이미지를 내려받아 `1/1 Running`이 되었다. 기존 Pod는 새 Pod의 Readiness가 확인된 뒤 순차적으로 종료되었다.

```bash
kubectl rollout status deployment/chat-app \
  -n chat \
  --timeout=120s
```

```text
deployment "chat-app" successfully rolled out
```

실행 이미지 확인:

```bash
kubectl get pods \
  -n chat \
  -l app=chat-app \
  -o custom-columns='NAME:.metadata.name,IMAGE:.spec.containers[0].image,READY:.status.containerStatuses[0].ready'
```

---

## 이미지를 비공개로 유지해야 한다면

이번 프로젝트에서는 외부에 공개해도 되는 포트폴리오 이미지였기 때문에 GHCR Package를 `Public`으로 변경하였다.

만약 Image를 공개할 수 없어 `Private` 상태로 유지해야 한다면, Package를 공개하는 대신 Kubernetes가 GHCR에 인증할 수 있도록 Registry 전용 Secret을 구성해야 한다.

```text
Kubernetes kubelet
→ Registry Secret에서 GHCR 인증정보 확인
→ Private GHCR Package 인증
→ Image Pull
→ Container 실행
```

현재 프로젝트의 `postgres-secret`은 Application과 PostgreSQL이 Database에 접속할 때 사용하는 Secret이다. Private Image Pull에 필요한 인증정보와는 목적과 사용 시점이 다르므로 그대로 사용할 수 없다.

```text
postgres-secret
→ Container 실행 후 DB 접속에 사용

ghcr-pull-secret
→ Container 실행 전 kubelet이 Private Image Pull에 사용
```

### Registry 읽기 전용 인증정보 준비

Kubernetes는 Image를 내려받기만 하면 되므로 GHCR Package를 읽을 수 있는 최소 권한의 인증정보를 사용해야 한다.

```text
GitHub Actions의 GITHUB_TOKEN
→ Docker Image Push

Kubernetes의 GHCR Read Token
→ Private Docker Image Pull
```

CI의 Push 권한과 Kubernetes의 Pull 권한을 분리하면 Kubernetes에 불필요한 Package 쓰기 권한을 제공하지 않을 수 있다.

### Kubernetes Registry Secret 생성

Private GHCR Package를 읽을 수 있는 인증정보로 `kubernetes.io/dockerconfigjson` 타입의 Secret을 `chat` Namespace에 생성할 수 있다.

```bash
kubectl create secret docker-registry ghcr-pull-secret \
  --namespace=chat \
  --docker-server=ghcr.io \
  --docker-username=<GHCR_USERNAME> \
  --docker-password=<GHCR_READ_TOKEN>
```

Secret은 Namespace에 속하므로 Image를 사용하는 Pod와 같은 `chat` Namespace에 존재해야 한다.

```bash
kubectl get secret ghcr-pull-secret -n chat
```

### Pod Template에 imagePullSecrets 연결

`chat-deployment.yaml`의 Pod Template에서 Registry Secret을 참조하도록 구성할 수 있다.

```yaml
spec:
  template:
    spec:
      imagePullSecrets:
        - name: ghcr-pull-secret
      containers:
        - name: chat-app
          image: ghcr.io/pang-4/chat-app:<COMMIT_SHA>
```

Migration Job도 같은 Private Image를 사용하므로 Job의 Pod Template에도 동일한 `imagePullSecrets` 설정이 필요하다.

```yaml
spec:
  template:
    spec:
      imagePullSecrets:
        - name: ghcr-pull-secret
```

이 인증정보는 Go Application에 환경변수로 전달되는 값이 아니다. Container를 실행하기 전에 Node의 kubelet이 Registry 인증과 Image Pull에 사용한다.

### 적용 시 확인할 항목

```bash
kubectl rollout status deployment/chat-app \
  -n chat \
  --timeout=120s
```

Image Pull에 계속 실패한다면 Pod Events를 확인한다.

```bash
kubectl describe pod <NEW_CHAT_POD> -n chat
```

오류별 확인 대상:

```text
unauthorized
→ Username, Token과 Package 읽기 권한 확인

pull access denied
→ GHCR Package 접근 권한 확인

FailedToRetrieveImagePullSecret
→ Secret 이름과 Namespace 확인

manifest unknown
→ Image 이름과 Tag 존재 여부 확인
```

### Secret 관리 시 주의할 점

Kubernetes Secret은 값을 Base64로 표현할 뿐, 그 자체로 안전하게 암호화된 비밀번호 저장소라고 볼 수 없다. 따라서 실제 GHCR Token을 포함한 Secret Manifest를 Git에 Commit하면 안 된다.

개인 실습에서는 `kubectl`로 Secret을 직접 생성할 수 있지만, Secret을 지속적으로 관리해야 하는 환경에서는 다음과 같은 방식을 검토할 수 있다.

- 전용 계정 또는 전용 인증수단 사용
- Image Pull에 필요한 최소 읽기 권한만 부여
- Token 만료와 교체 주기 관리
- External Secrets와 Secret Manager 또는 Vault 연동
- EKS와 ECR 환경에서는 Node 또는 실행 Role의 IAM Pull 권한 사용

이 내용은 이번 프로젝트에서 직접 적용하여 검증한 결과가 아니라, Image를 비공개로 유지해야 한다면 어떻게 구성할 수 있는지 검토한 운영 대안이다.

---

## 배운 점

- CI의 이미지 Push 성공과 Kubernetes의 이미지 Pull 성공은 별개의 검증이다.
- `ImagePullBackOff`에서는 `describe`의 Events와 Registry 권한을 확인한다.
- Rolling Update는 새 Pod가 실패하면 기존 정상 Pod를 유지해 전체 중단을 방지한다.
- 공개할 수 없는 이미지는 Package를 Public으로 변경하지 않고 Kubernetes `imagePullSecrets` 또는 Cloud IAM으로 인증한다.
- Database 인증용 Secret과 Registry 인증용 Secret은 사용 주체와 목적이 다르다.
- CI의 Image Push 권한과 Kubernetes의 Image Pull 권한은 분리하고 최소 권한을 부여한다.
- 실제 Token이 포함된 Secret Manifest는 Git에 Commit하지 않는다.
