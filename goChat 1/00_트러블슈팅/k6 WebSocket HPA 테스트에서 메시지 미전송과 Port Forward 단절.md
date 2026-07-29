# k6 WebSocket HPA 테스트에서 메시지 미전송과 Port Forward 단절

## 문제 개요

300명 HPA 부하 테스트를 준비하면서 서로 다른 두 문제가 발생하였다.

1. WebSocket 연결과 입장 Message는 성공했지만 실제 Chat Message가 전송되지 않았다.
2. 300명 테스트 중 `kubectl port-forward`가 끊기면서 Message 전달 검증이 실패했다.

두 문제 모두 Chat Application 자체의 연결 실패처럼 보일 수 있지만 원인은 각각 k6 Script와 외부 접속 경로에 있었다.

---

## 문제 1: 연결은 성공하지만 Chat Message가 전송되지 않음

### 증상

![[01. 10명 메시지 미전송 실패.png]]

10명 테스트 결과:

```text
connection_success: 100% (10/10)
join broadcast received: 성공
message_delivery_success: 0%
ws_msgs_sent: 10
0 complete and 10 interrupted
```

경고:

```text
No script iterations fully finished,
consider making the test duration longer
```

`ws_msgs_sent=10`은 10명의 VU가 각각 보낸 `join` Message만 집계된 값이었다. 1초마다 보내야 하는 실제 `chat` Message는 전송되지 않았다.

### 잘못된 초기 판단

처음에는 다음 원인을 의심할 수 있었다.

- 테스트 `maxDuration`이 너무 짧음
- 서버가 Message를 처리하지 못함
- Ingress WebSocket Timeout
- PostgreSQL 또는 Redis 오류

하지만 모든 VU가 입장 알림을 수신했고 연결도 유지되었다. 서버 연결보다 k6의 Message 전송 Timer가 실행되지 않은 이유를 먼저 확인해야 했다.

### 원인

`k6/websockets`로 생성한 Socket의 상태를 다음과 같이 검사하고 있었다.

```javascript
if (socket.readyState !== WebSocket.OPEN) {
  stopTimers();
  return;
}
```

현재 실행 환경에서 `WebSocket.OPEN` 비교가 기대대로 동작하지 않았다.

첫 번째 전송 Timer가 실행될 때 Socket을 열린 상태로 판정하지 못했고, `stopTimers()`가 전송 Timer뿐 아니라 종료 Timer까지 제거하였다.

그 결과:

```text
Connection은 계속 열려 있음
Chat Message 전송 Timer는 제거됨
정상 종료 Timer도 제거됨
maxDuration까지 대기
모든 VU가 interrupted 처리됨
```

### 해결

WebSocket 표준의 `OPEN` 상태 값인 `1`을 명시적으로 정의하여 `readyState`와 비교하였다.

```javascript
const websocketOpenState = 1;
```

전송 전 상태 확인:

```javascript
if (socket.readyState !== websocketOpenState) {
  stopTimers();
  return;
}
```

정상 종료 전 상태 확인:

```javascript
if (socket.readyState === websocketOpenState) {
  socket.close();
}
```

수정 파일:

```text
loadtest/websocket-load.js
```

### 수정 후 10명 검증

![[02. 10명 스크립트 수정 후 성공.png]]

```bash
k6 run \
  -e WS_URL=ws://100.126.215.15:8080/ws \
  -e VUS=10 \
  -e RAMP_SECONDS=10 \
  -e HOLD_SECONDS=30 \
  -e MESSAGE_INTERVAL_MS=1000 \
  loadtest/websocket-hpa.js
```

결과:

```text
Check: 708/708 성공
Connection 성공률: 100% (10/10)
Message 전달 성공률: 100% (10/10)
Message 지연 평균: 15.02ms
Message 지연 p95: 22ms
10 complete and 0 interrupted
```

실제 Chat Message가 반복 전송되었고 모든 VU가 정상 종료되었다.

---

## 문제 2: 300명 테스트 중 Port Forward 연결이 끊김

### 증상

![[03. 300명 Port Forward 전달 실패.png]]

300명 테스트 중 Mini PC의 Port Forward Terminal에서 다음 오류가 발생하였다.

![[06. Port Forward Broken Pipe.png]]

```text
Unhandled Error
write: broken pipe
error: lost connection to pod
```

k6 결과:

```text
Connection 성공률: 100% (300/300)
Message 전달 성공률: 0% (0/300)
300 complete and 0 interrupted
```

HPA 결과:

![[04. 실패 테스트 HPA 3에서 6 확장.png]]

```text
CPU: 3% → 28% → 83% → 157% → 209%
Pod: 3개 → 4개 → 6개
```

![[05. 실패 테스트 신규 Pod 생성.png]]

Grafana에서는 WebSocket Connection이 증가하다가 Port Forward 오류 시점에 한꺼번에 0으로 떨어졌다.

![[07. Port Forward 단절 시 연결 급락.png]]

### 원인

테스트 경로:

```text
Mac k6
  → kubectl port-forward
  → ingress-nginx-controller
  → Chat Service
  → Chat Pod
```

300명의 사용자가 1초마다 Message를 보내고, 각 Message가 같은 Room의 사용자들에게 Broadcast되었다.

테스트 중 k6가 수신한 데이터는 약 `1.7GB`까지 증가하였다. 이 Traffic을 전달하던 임시 Port Forward 경로가 `broken pipe`로 종료되면서 모든 WebSocket Connection이 끊겼다.

`kubectl port-forward`는 개발과 짧은 점검을 위한 임시 연결 방식이다. 이번 수준의 장시간 다중 WebSocket 부하 테스트를 위한 정식 Load Balancer가 아니다.

HPA가 6개까지 정상 확장한 점과 300개 Connection이 최초에는 모두 성공한 점을 보면 Application의 최초 연결 실패가 아니라 외부 전달 경로의 단절이었다.

### 해결 방향

Ingress Controller Service가 이미 NodePort로 노출되어 있었다.

```bash
kubectl get svc ingress-nginx-controller -n ingress-nginx -o wide
minikube ip
```

확인 결과:

```text
Minikube IP: 192.168.49.2
HTTP NodePort: 31673
Tailscale IP: 100.126.215.15
```

Mini PC 내부에서는 다음 주소로 Ingress에 직접 접근할 수 있었다.

```bash
curl --max-time 5 http://192.168.49.2:31673/health/live
```

결과:

```text
ok
```

그러나 Minikube IP는 Mini PC 내부 Network이므로 Mac에서 직접 접근하기 어렵다.

Mini PC의 Tailscale 주소와 Ingress NodePort 사이에 `socat` TCP 중계를 구성하였다.

```bash
socat \
  TCP-LISTEN:8080,bind=100.126.215.15,reuseaddr,fork \
  TCP:192.168.49.2:31673
```

변경된 경로:

```text
Mac k6
  → Tailscale 100.126.215.15:8080
  → socat
  → Minikube 192.168.49.2:31673
  → NGINX Ingress
  → Chat Service
  → Chat Pod
```

`fork` 옵션은 들어오는 TCP Connection마다 전달 처리를 생성한다. WebSocket도 TCP 위에서 동작하므로 이 경로를 그대로 사용할 수 있다.

### 경로 변경 후 사전 검증

![[08. socat 경로 10명 검증 성공.png]]

동일한 k6 Script를 10명으로 다시 실행하였다.

```text
Connection 성공률: 100%
Message 전달 성공률: 100%
10 complete and 0 interrupted
```

이를 통해 `socat → Ingress NodePort` 경로에서 WebSocket 접속, Message 전송, Broadcast 수신과 정상 종료가 모두 가능함을 확인하였다.

### 300명 최종 재검증

![[09. 300명 k6 최종 성공.png]]

10명 사전 검증 이후 같은 접속 경로로 300명 HPA 테스트를 다시 실행하였다.

```text
Connection 성공률: 100% (300/300)
Message 전달 성공률: 100% (300/300)
Check: 225,290/225,290 성공
Check 실패: 0
300 complete and 0 interrupted
```

처리량:

```text
송신 Message: 112,945개
수신 Message: 29,361,590개
수신 데이터: 4.0GB
```

HPA:

```text
CPU 최대: 247%
Chat Pod: 3개 → 6개
```

Network Packet Drop은 0이었으며 `broken pipe`와 Port Forward 연결 단절도 재발하지 않았다.

따라서 다음 두 문제는 모두 해결된 것으로 판정하였다.

```text
Chat Message 미전송 문제: 해결
대규모 테스트 외부 연결 단절 문제: 해결
```

HPA Scale Down은 부하 종료 후 5분 안정화 시간을 거쳐 `6 → 4 → 3`으로 복귀하였다.

![[19. HPA 6에서 3 축소 완료.png]]

---

## Grafana 해석 시 주의점

다음 Query는 Pod별 Connection을 각각 표시한다.

```promql
chat_websocket_connections
```

여러 선이 각각 약 100을 표시하면 전체가 100명이라는 뜻이 아니다. Pod별 값을 합산해야 전체 Connection 수를 확인할 수 있다.

전체 연결 수:

```promql
sum(chat_websocket_connections)
```

HPA 확장으로 새 Pod가 생성되어도 기존 WebSocket Connection은 새 Pod로 이동하지 않는다. 새 Pod가 준비된 이후 생성되는 신규 Connection부터 분산된다.

---

## 문제를 구분하는 기준

| 관찰 결과 | 가능성이 높은 원인 |
|---|---|
| 연결 성공, `ws_msgs_sent`가 VU 수와 같음 | 입장 Message만 전송되고 Chat Timer가 동작하지 않음 |
| `0 complete`, 전체 `interrupted` | 종료 Timer가 동작하지 않거나 `maxDuration` 부족 |
| `broken pipe`, `lost connection to pod` | Port Forward 전달 경로 단절 |
| Grafana 연결 수가 동시에 0으로 감소 | 외부 연결 경로 또는 Ingress Connection 단절 |
| HPA는 확장했지만 전달 성공률 0% | 부하 생성은 됐으나 테스트 도중 Connection이 끊김 |
| `connection_success=0%` | URL, Port, Ingress, Network 또는 서버 최초 연결 문제 |

---

## 최종 정리

```text
첫 번째 실패 원인:
WebSocket 상태 비교 오류로 전송·종료 Timer가 제거됨

첫 번째 해결:
readyState의 OPEN 값 1을 명시적으로 비교

두 번째 실패 원인:
대량 WebSocket Broadcast Traffic 중 kubectl port-forward 단절

두 번째 해결:
socat으로 Tailscale 주소를 Ingress NodePort에 직접 중계

최종 사전 검증:
10명 Connection과 Message 전달 모두 100%
10 complete and 0 interrupted

최종 본 검증:
300명 Connection과 Message 전달 모두 100%
225,290/225,290 Check 성공
300 complete and 0 interrupted
HPA 3개에서 6개로 확장
Packet Drop 0
```

운영 환경에서는 `socat`도 임시 중계 수단이다. 실제 서비스에서는 외부 IP가 할당된 Ingress, MetalLB 또는 Cloud LoadBalancer와 DNS·TLS를 구성해야 한다.
