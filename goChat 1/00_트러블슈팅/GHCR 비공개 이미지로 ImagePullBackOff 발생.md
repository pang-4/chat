# GHCR 비공개 이미지로 ImagePullBackOff 발생

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

## 배운 점

- CI의 이미지 Push 성공과 Kubernetes의 이미지 Pull 성공은 별개의 검증이다.
- `ImagePullBackOff`에서는 `describe`의 Events와 Registry 권한을 확인한다.
- Rolling Update는 새 Pod가 실패하면 기존 정상 Pod를 유지해 전체 중단을 방지한다.
- 비공개 이미지를 유지해야 한다면 Package를 공개하는 대신 Kubernetes `imagePullSecret`을 사용한다.
