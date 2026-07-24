# Minikube 동일 이미지 태그 재사용으로 이전 이미지가 배포된 문제

## 문제

Go 채팅 서버에 Redis Pub/Sub 기능을 추가한 뒤 새로운 Docker 이미지를 빌드하고 Minikube의 Chat Deployment를 재시작하였다.

```bash
docker build \
  -f docker/Dockerfile \
  -t chat-app:local \
  app

minikube image load chat-app:local

kubectl rollout restart deployment/chat-app -n chat
```

Deployment의 롤링 업데이트는 성공했고 Chat Pod 3개도 모두 `1/1 Running` 상태였다.

하지만 애플리케이션 로그에서 새 코드에 추가한 다음 로그가 나오지 않았다.

```text
redis subscription started
```

대신 기존 코드와 동일하게 서버 시작 로그와 Kubernetes 상태 검사 로그만 출력되었다.

```text
server started
GET /health/ready
GET /health/live
```

---

## 원인 확인

현재 Pod가 실행하는 이미지 ID를 확인하였다.

```bash
kubectl get pods \
  -n chat \
  -l app=chat-app \
  -o custom-columns='NAME:.metadata.name,IMAGE:.spec.containers[0].image,IMAGE_ID:.status.containerStatuses[0].imageID'
```

세 Pod는 모두 다음 이전 이미지 ID를 사용하고 있었다.

```text
IMAGE              IMAGE_ID
chat-app:local     docker://sha256:ae3592...
```

Ubuntu Docker에서 새로 빌드한 이미지 ID도 확인하였다.

```bash
docker image inspect chat-app:local \
  --format '{{.Id}}'
```

새 이미지의 ID는 Pod에서 사용 중인 이미지와 달랐다.

```text
Ubuntu Docker 이미지: sha256:d8a312...
Minikube Pod 이미지:  sha256:ae3592...
```

따라서 Git Pull과 Docker 이미지 빌드는 정상적으로 완료됐지만, Kubernetes Pod에는 새로운 이미지가 적용되지 않았다는 사실을 확인하였다.

---

## 원인

Ubuntu Docker와 Minikube 노드의 컨테이너 런타임은 서로 분리되어 있다.

```text
Ubuntu Docker
└── chat-app:local → 새 이미지

Minikube 노드
└── chat-app:local → 이전 이미지
```

두 환경에 이름과 태그가 동일한 `chat-app:local` 이미지가 존재했지만 실제 이미지 내용과 ID는 달랐다.

Chat Deployment에는 다음 설정이 있었다.

```yaml
image: chat-app:local
imagePullPolicy: IfNotPresent
```

`IfNotPresent`는 해당 이름과 태그의 이미지가 Minikube 노드에 이미 있으면 기존 로컬 이미지를 사용할 수 있게 한다. Minikube에는 이전 `chat-app:local` 이미지가 존재했으므로 새 Pod를 생성해도 이전 이미지가 사용되었다.

또한 다음 명령은 새 이미지를 빌드하거나 이미지 내용을 비교하는 명령이 아니다.

```bash
kubectl rollout restart deployment/chat-app -n chat
```

이 명령은 Deployment의 Pod를 다시 생성할 뿐이며, Deployment가 가리키는 이미지 태그가 그대로라면 같은 이전 이미지가 다시 실행될 수 있다.

---

## 해결 과정

동일한 `local` 태그를 다시 사용하지 않고 Redis 기능이 포함된 이미지에 새로운 태그를 붙였다.

```bash
docker tag chat-app:local chat-app:redis-v1
```

새 태그가 Ubuntu Docker의 새 이미지를 가리키는지 확인하였다.

```bash
docker image inspect chat-app:redis-v1 \
  --format '{{.Id}}'
```

새 태그의 이미지를 Minikube에 전달하였다.

```bash
minikube image load \
  --daemon \
  chat-app:redis-v1
```

Minikube 내부에 이미지가 등록되었는지 확인하였다.

```bash
minikube ssh -- \
  docker image inspect chat-app:redis-v1 \
  --format '{{.Id}}'
```

Deployment가 새 태그를 사용하도록 변경하였다.

```bash
kubectl set image \
  deployment/chat-app \
  chat-app=chat-app:redis-v1 \
  -n chat
```

롤링 업데이트가 완료될 때까지 기다렸다.

```bash
kubectl rollout status deployment/chat-app \
  -n chat \
  --timeout=120s
```

---

## 결과 확인

새로 생성된 Chat Pod 3개가 모두 정상 실행되는 것을 확인하였다.

```bash
kubectl get pods -n chat
```

```text
chat-app-...   1/1   Running   0
chat-app-...   1/1   Running   0
chat-app-...   1/1   Running   0
postgres-0     1/1   Running   0
redis-...      1/1   Running   0
```

Pod가 사용하는 이미지 태그와 이미지 ID도 확인하였다.

```bash
kubectl get pods \
  -n chat \
  -l app=chat-app \
  -o custom-columns='NAME:.metadata.name,IMAGE:.spec.containers[0].image,IMAGE_ID:.status.containerStatuses[0].imageID'
```

세 Pod 모두 `chat-app:redis-v1` 이미지를 사용하였다.

```text
IMAGE
chat-app:redis-v1
chat-app:redis-v1
chat-app:redis-v1
```

애플리케이션 시작 로그를 확인하였다.

```bash
kubectl logs \
  pod/<CHAT_POD_NAME> \
  -n chat \
  | grep -E 'redis|server started|ERROR'
```

다음 로그가 출력되었다.

```text
redis subscription started  channel=chat:messages
server started              address=:8080
```

이 로그를 통해 Chat Pod가 Redis의 `chat:messages` 채널을 구독한 뒤 WebSocket 서버를 시작했다는 것을 확인하였다.

---

## 로그 확인 시 주의점

다음 명령은 가장 최근 로그 30줄만 출력한다.

```bash
kubectl logs pod/<CHAT_POD_NAME> -n chat --tail=30
```

Redis 구독 로그는 애플리케이션 시작 시 한 번만 출력된다. 이후 Kubernetes가 `/health/ready`와 `/health/live`를 반복 호출하면 시작 로그가 최근 30줄 밖으로 밀려날 수 있다.

따라서 시작 로그를 확인할 때는 전체 로그에서 검색한다.

```bash
kubectl logs pod/<CHAT_POD_NAME> -n chat \
  | grep -E 'redis|server started|ERROR'
```

---

## Git 선언 상태와 실제 상태 맞추기

`kubectl set image`는 실행 중인 Kubernetes Deployment만 변경한다. Git의 `k8s/chat-deployment.yaml`이 계속 다음 값을 가지고 있으면 실제 클러스터 상태와 Git 선언이 달라진다.

```yaml
image: chat-app:local
```

동작 확인 후에는 YAML도 실제 배포 태그와 동일하게 수정해야 한다.

```yaml
image: chat-app:redis-v1
```

이후 YAML을 GitHub에 커밋하여 Git의 선언 상태와 Kubernetes의 실제 상태를 일치시킨다.

---

## 재발 방지

개발 중이라도 내용이 달라진 컨테이너 이미지에 동일한 태그를 반복해서 사용하는 방식은 피한다.

학습 단계에서는 기능이나 버전을 나타내는 태그를 사용할 수 있다.

```text
chat-app:v1
chat-app:redis-v1
chat-app:redis-v2
```

CI/CD를 구성한 뒤에는 Git 커밋 SHA처럼 중복되지 않는 태그를 자동으로 생성하는 것이 좋다.

```text
chat-app:3d78c2a
chat-app:a82f119
```

고유한 이미지 태그를 사용하면 다음 내용을 명확하게 알 수 있다.

- 어떤 소스 코드로 생성한 이미지인지
- Kubernetes가 현재 어떤 버전을 실행하는지
- 새로운 버전이 실제로 배포되었는지
- 장애 발생 시 어떤 버전으로 롤백해야 하는지

---

## 배운 점

- Ubuntu Docker와 Minikube 내부 컨테이너 런타임은 서로 다른 이미지 저장소를 사용한다.
- 같은 이미지 이름과 태그라도 환경에 따라 실제 이미지 ID가 다를 수 있다.
- `kubectl rollout restart`는 이미지를 새로 빌드하거나 태그를 갱신하지 않는다.
- `imagePullPolicy: IfNotPresent`에서는 노드에 같은 태그가 있으면 기존 이미지가 사용될 수 있다.
- 새로운 코드를 배포할 때는 고유한 이미지 태그를 사용하는 것이 안전하다.
- `kubectl set image`로 변경한 실제 상태는 Kubernetes YAML에도 반영해야 한다.
- Pod가 `Running`이라고 해서 반드시 원하는 애플리케이션 버전이 실행 중인 것은 아니다.
- 이미지 ID와 애플리케이션 시작 로그를 함께 확인해야 배포 버전을 정확히 검증할 수 있다.
