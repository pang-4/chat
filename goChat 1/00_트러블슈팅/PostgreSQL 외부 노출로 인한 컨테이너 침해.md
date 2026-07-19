# PostgreSQL 외부 노출로 인한 컨테이너 침해

## 문제

Proxmox에서 `chat` 서버의 CPU 사용량이 비정상적으로 높게 나타났다.

```text
CPU usage: 약 100% of 8 CPU(s)
load average: 8.24, 8.15, 8.16
```

컨테이너별 사용량을 확인한 결과 Go 채팅 서버는 정상 상태였지만 PostgreSQL 컨테이너가 8개 CPU를 거의 모두 사용하고 있었다.

```text
chat-app        CPU 0.00%
chat-postgres   CPU 약 790%
```

PostgreSQL 컨테이너를 중지하자 CPU 사용량이 정상으로 돌아왔다.

---

## 원인 확인

다음 명령으로 PostgreSQL 컨테이너 내부의 프로세스를 확인하였다.

```bash
docker top chat-postgres -eo pid,ppid,user,comm,args
```

정상적인 PostgreSQL 프로세스 외에 다음과 같은 의심스러운 프로세스가 실행되고 있었다.

```text
earnfm 관련 프로세스
pawns-cli
정체불명의 cli 및 Node.js 프로세스
```

이 프로세스들은 PostgreSQL의 정상 구성요소가 아니다. 프로세스 인자에는 외부 계정과 토큰도 포함되어 있었으므로 문서와 GitHub에는 실제 값을 기록하지 않았다.

당시 Compose 설정은 PostgreSQL 포트를 다음과 같이 공개하고 있었다.

```yaml
ports:
  - "5432:5432"
```

이 설정은 Ubuntu의 모든 네트워크 인터페이스에 PostgreSQL 포트를 공개한다. 서버에 공인 IP가 있고 DB 비밀번호도 추측하기 쉬운 값이었기 때문에 외부에서 PostgreSQL에 접근하여 컨테이너를 악용한 것으로 강하게 추정하였다.

---

## 긴급 조치

추가적인 CPU 및 네트워크 사용을 막기 위해 감염된 PostgreSQL 컨테이너를 즉시 중지하였다.

```bash
docker stop chat-postgres
```

중지 후 다음 명령으로 호스트의 CPU 사용량이 정상으로 돌아온 것을 확인하였다.

```bash
docker stats --no-stream
ps -eo pid,comm,args,%cpu,%mem --sort=-%cpu | head -15
```

감염된 컨테이너는 다시 시작하지 않았다.

---

## Compose 보안 설정 변경

PostgreSQL은 Go 서버와 Migration에서만 사용하므로 호스트에 `5432` 포트를 공개할 필요가 없다. `ports` 설정을 제거하고 Compose 내부 네트워크에서만 접근하도록 변경하였다.

```yaml
postgres:
  image: postgres:17
  environment:
    POSTGRES_USER: ${DB_USER:-postgres}
    POSTGRES_PASSWORD: ${DB_PASSWORD:?DB_PASSWORD is required}
    POSTGRES_DB: ${DB_NAME:-chat}
  volumes:
    - postgres-data:/var/lib/postgresql/data
```

Go 서버와 Migration은 다음 내부 주소로 PostgreSQL에 연결한다.

```text
postgres:5432
```

외부에 공개되는 포트는 Go 채팅 서버의 `8080`뿐이다.

---

## DB 비밀번호 변경

기존 비밀번호는 더 이상 안전하다고 볼 수 없으므로 새로운 비밀번호를 생성하였다.

```bash
openssl rand -hex 32
```

생성된 값을 Ubuntu의 `app/.env`에 입력한다.

```env
DB_PASSWORD=새로_생성한_비밀번호
```

실제 비밀번호가 들어 있는 `app/.env`는 `.gitignore`로 제외한다. GitHub에 올리는 `app/.env.example`에는 변수 이름만 남기고 비밀번호 값은 비워 둔다.

```env
DB_PASSWORD=
```

---

## 감염 컨테이너와 DB Volume 재생성

기존 채팅 데이터가 필요하지 않았기 때문에 감염된 컨테이너와 PostgreSQL Volume을 모두 삭제하였다.

> `-v`는 PostgreSQL 데이터까지 삭제하므로 데이터 보존이 필요하면 먼저 별도의 복구 절차를 진행해야 한다.

```bash
cd ~/go-devops
docker compose --env-file app/.env -f docker/compose.yml down -v --remove-orphans
```

PostgreSQL 공식 이미지를 다시 확인하고 내려받는다.

```bash
docker pull postgres:17
```

Docker 이미지는 읽기 전용 원본이므로 일반적인 컨테이너 침해가 이미지 레이어를 직접 변경하지는 않는다. 중요한 것은 감염된 컨테이너의 쓰기 영역과 DB Volume을 제거하는 것이다. 로컬 이미지까지 완전히 다시 받고 싶다면 컨테이너 삭제 후 다음과 같이 실행할 수 있다.

```bash
docker image rm postgres:17
docker pull postgres:17
```

새 비밀번호와 깨끗한 Volume으로 전체 서비스를 다시 실행한다.

```bash
docker compose --env-file app/.env -f docker/compose.yml up -d --build
```

Compose는 다음 순서로 서비스를 준비한다.

```text
PostgreSQL 생성
↓
Migration 실행
↓
Go 채팅 서버 실행
```

---

## 결과 확인

컨테이너 상태를 확인한다.

```bash
docker ps
```

PostgreSQL의 PORTS가 다음처럼 내부 포트로만 표시되어야 한다.

```text
5432/tcp
```

다음처럼 표시되면 여전히 외부에 공개된 상태이므로 Compose 설정을 다시 확인해야 한다.

```text
0.0.0.0:5432->5432/tcp
```

프로세스와 자원 사용량도 확인한다.

```bash
docker top chat-postgres -eo pid,ppid,user,comm,args
docker stats --no-stream
```

PostgreSQL 컨테이너에는 `postgres`, `checkpointer`, `walwriter`, `autovacuum` 등 정상적인 PostgreSQL 프로세스만 있어야 한다.

---

## 배운 점

- 데이터베이스 포트는 외부 접근이 반드시 필요한 경우가 아니라면 공개하지 않는다.
- 포트 번호만 변경하는 것은 보안 대책이 아니다. 외부 포트 연결 자체를 제거해야 한다.
- 기본 계정과 추측하기 쉬운 비밀번호를 사용하지 않는다.
- 실제 비밀번호는 `.env`에 저장하고 GitHub에 커밋하지 않는다.
- `.env.example`에는 환경변수 이름과 안전한 예시값만 기록한다.
- 컨테이너를 삭제해도 Volume이 남으면 데이터와 의심스러운 파일이 유지될 수 있다.
- 침해가 의심되는 환경은 단순 재시작하지 않고 컨테이너와 Volume을 깨끗하게 재생성한다.
- `docker stats`, `docker top`, `ps`, `uptime`을 함께 사용하면 자원 과부하의 원인을 좁힐 수 있다.
