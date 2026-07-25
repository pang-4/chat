# ArgoCD 비공개 GitHub 저장소 인증 실패

## 문제

ArgoCD `Application`을 생성한 뒤 상태를 확인했지만 `SYNC STATUS`가 `Unknown`으로 표시되었다.

```bash
kubectl get application chat -n argocd -w
```

```text
NAME   SYNC STATUS   HEALTH STATUS
chat   Unknown       Healthy
```

`Healthy`이므로 ArgoCD Application 객체는 존재하지만, Git과 Kubernetes의 상태를 비교하지 못하고 있는 상황이었다.

---

## 원인 확인

Application의 상세 정보와 `Conditions`를 확인하였다.

```bash
kubectl describe application chat -n argocd
```

다음 오류가 기록되어 있었다.

```text
Failed to load target state:
failed to generate manifest:
failed to list refs:
authentication required:
Repository not found.
```

ArgoCD Application에는 다음 GitHub 저장소가 등록되어 있었다.

```text
https://github.com/pang-4/chat.git
```

하지만 GitHub 저장소가 `Private`이었고, ArgoCD에는 저장소를 읽을 인증 정보가 없었다.

```text
ArgoCD
→ 비공개 GitHub 저장소 접근
→ 인증 정보 없음
→ k8s 매니페스트를 읽지 못함
→ 현재 상태와 원하는 상태를 비교할 수 없음
→ Sync Unknown
```

---

## 해결 전 보안 확인

포트폴리오 저장소를 공개하기 전에 실제 비밀번호나 토큰이 Git에 포함되지 않았는지 확인하였다.

```bash
git grep -n -i -E \
  'password|token|secret|api[_-]?key|private[_-]?key'

git status --ignored
```

확인 결과:

- 실제 값을 가진 `app/.env`는 `.gitignore`에 의해 제외되어 있었다.
- `app/.env.example`에는 변수 이름만 있고 실제 값은 없었다.
- `${{ secrets.GITHUB_TOKEN }}`은 GitHub Actions 실행 중 GitHub가 제공하는 임시 토큰 참조였다.
- Kubernetes YAML에는 실제 비밀번호 대신 `secretKeyRef`만 기록되어 있었다.
- Git 기록에도 실제 `.env` 파일이 포함되지 않았다.

---

## 해결

이 저장소는 채용 포트폴리오로 공개할 목적이므로 GitHub Repository의 Visibility를 `Public`으로 변경하였다.

```text
GitHub Repository
→ Settings
→ General
→ Danger Zone
→ Change repository visibility
→ Public
```

그다음 ArgoCD가 Git 저장소를 즉시 다시 읽도록 Hard Refresh를 요청하였다.

```bash
kubectl annotate application chat \
  -n argocd \
  argocd.argoproj.io/refresh=hard \
  --overwrite
```

상태 변화를 확인하였다.

```bash
kubectl get application chat -n argocd -w
```

---

## 결과

상태가 다음 순서로 변경되었다.

```text
Unknown
→ OutOfSync
→ Synced
```

최종 결과:

```text
NAME   SYNC STATUS   HEALTH STATUS
chat   Synced        Healthy
```

- `OutOfSync`: Git에 기록된 원하는 상태와 Kubernetes의 실제 상태에 차이가 있음을 의미한다.
- `Synced`: ArgoCD가 차이를 Kubernetes에 반영하여 Git과 실제 상태가 같아졌음을 의미한다.
- `Healthy`: 배포된 Kubernetes 리소스가 정상적으로 동작하고 있음을 의미한다.

---

## 다른 해결 방법

비공개 저장소를 유지해야 한다면 저장소를 공개하는 대신 ArgoCD에 GitHub 인증 정보를 등록할 수 있다.

```text
ArgoCD
→ Settings
→ Repositories
→ Connect Repo
→ VIA HTTPS
```

이때 저장소 읽기 권한만 가진 Fine-grained Personal Access Token을 사용하는 것이 안전하다. 토큰 값은 Git이나 문서에 기록하지 않는다.

---

## 배운 점

- `Healthy`와 `Synced`는 서로 다른 상태다.
- `Healthy`여도 Git 저장소를 읽지 못하면 `Sync Status`는 `Unknown`이 될 수 있다.
- `Unknown` 상태에서는 `kubectl describe application`의 `Conditions`를 먼저 확인한다.
- 비공개 Git 저장소를 ArgoCD에서 사용하려면 Repository 인증 정보가 필요하다.
- 공개 전환 전에 현재 파일뿐 아니라 Git 기록과 무시된 파일도 점검해야 한다.
- `Synced / Healthy`는 Git의 원하는 상태가 Kubernetes에 반영되었고 리소스도 정상이라는 뜻이다.
