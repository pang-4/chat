# Go WebSocket Chat Server

Go의 Goroutine과 Channel을 이용한 실시간 WebSocket 채팅 서버입니다. 현재 단계에서는 Ubuntu에서 PostgreSQL을 Docker로 실행하고 Go 서버는 호스트에서 실행합니다.

## 현재 구현 범위

- 다중 WebSocket 연결 관리
- 입장, 채팅, 퇴장 메시지 Broadcast
- PostgreSQL 채팅 메시지 저장
- 버전 기반 PostgreSQL 스키마 마이그레이션
- 클라이언트별 제한된 송신 큐
- 느린 클라이언트 자동 연결 해제
- Ping/Pong 기반 죽은 연결 정리
- 메시지 크기 및 입력값 검증
- JSON 구조화 로그
- Liveness, readiness, 접속자 수 Endpoint
- SIGINT/SIGTERM graceful shutdown

## 실행 준비

필요한 프로그램:

- Go 1.25 이상
- Docker와 Docker Compose

Ubuntu에서 저장소를 받은 뒤 PostgreSQL을 실행합니다.

```bash
docker compose -f docker/compose.yml up -d
```

환경설정 파일을 준비합니다.

```bash
cp app/.env.example app/.env
```

기본 Compose 설정을 그대로 사용한다면 예제 값으로 연결할 수 있습니다. 실제 운영용 비밀번호는 저장소에 커밋하지 않습니다.

## 데이터베이스 마이그레이션

Go 서버를 처음 실행하거나 새로운 마이그레이션을 받은 경우 먼저 마이그레이션을 적용합니다.

```bash
cd app
go run ./cmd/migrate up
```

현재 버전 확인과 최근 마이그레이션 한 단계 롤백은 다음 명령을 사용합니다.

```bash
go run ./cmd/migrate version
go run ./cmd/migrate down
```

`down`은 테이블이나 데이터가 제거될 수 있으므로 개발 환경에서만 신중하게 실행합니다. 이미 이전 버전 서버가 `messages` 테이블을 만들었다면 첫 `up` 실행은 기존 테이블을 유지하고 마이그레이션 버전만 기록합니다.

## 서버 실행

```bash
cd app
go mod download
go run ./cmd/migrate up
go run ./cmd/server
```

기본 주소는 `http://localhost:8080`이며 WebSocket Endpoint는 `ws://localhost:8080/ws`입니다.

## WebSocket 프로토콜

연결 후 먼저 입장 메시지를 보냅니다.

```json
{"type":"join","sender":"alice","content":""}
```

그다음 채팅 메시지를 보냅니다. 서버는 클라이언트가 보낸 `sender`를 신뢰하지 않고 입장할 때 등록한 이름을 사용합니다.

```json
{"type":"chat","sender":"","content":"안녕하세요"}
```

서버 응답 예시:

```json
{
  "id": 1,
  "type": "chat",
  "sender": "alice",
  "content": "안녕하세요",
  "createdAt": "2026-07-13T00:00:00Z"
}
```

## 운영 확인 Endpoint

```text
GET /health/live   프로세스 생존 확인
GET /health/ready  PostgreSQL 연결 준비 상태 확인
GET /stats         현재 WebSocket 연결 수 확인
```

확인 예시:

```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
curl http://localhost:8080/stats
```

## 테스트

```bash
cd app
go test ./...
go test -race ./...
```

WebSocket 통합 테스트는 실제 서버 대신 테스트 서버와 가짜 저장소를 사용하여 입장, Broadcast, 발신자 검증, DB 저장 흐름을 확인합니다.
