import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import { WebSocket } from 'k6/websockets';

const wsURLs = (__ENV.WS_URLS || __ENV.WS_URL || 'ws://localhost:8080/ws')
  .split(',')
  .map((url) => url.trim())
  .filter((url) => url.length > 0);
const vus = Number(__ENV.VUS || 10);
const holdSeconds = Number(__ENV.HOLD_SECONDS || 15);
const messageIntervalMs = Number(__ENV.MESSAGE_INTERVAL_MS || 1000);

const connectionSuccess = new Rate('connection_success');
const messageDeliverySuccess = new Rate('message_delivery_success');
const ownMessageLatency = new Trend('own_message_latency', true);

export const options = {
  scenarios: {
    concurrent_connections: {
      executor: 'per-vu-iterations',
      vus,
      iterations: 1,
      maxDuration: `${holdSeconds + 15}s`,
    },
  },

  thresholds: {
    checks: ['rate>0.99'],
    connection_success: ['rate>0.99'],
    message_delivery_success: ['rate>0.99'],
    ws_connecting: ['p(95)<500'],
  },
};

export default function (sessionHoldSeconds = holdSeconds) {
  // VU를 WebSocket 주소에 순서대로 분배한다.
  // VU 1 → 첫 번째 주소, VU 2 → 두 번째 주소, VU 3 → 세 번째 주소,
  // VU 4 → 다시 첫 번째 주소로 연결된다.
  const wsURL = wsURLs[(__VU - 1) % wsURLs.length];
  const socket = new WebSocket(wsURL);
  const username = `k6-user-${__VU}-${__ITER}`;

  let connected = false;
  let joined = false;
  let joinEvaluated = false;
  let deliveryEvaluated = false;
  let sentCount = 0;
  let receivedOwnCount = 0;
  let joinTimer;
  let sendTimer;
  let sessionTimer;
  const sentAt = new Map();

  const stopTimers = () => {
    if (joinTimer !== undefined) {
      clearTimeout(joinTimer);
      joinTimer = undefined;
    }

    if (sendTimer !== undefined) {
      clearInterval(sendTimer);
      sendTimer = undefined;
    }

    if (sessionTimer !== undefined) {
      clearTimeout(sessionTimer);
      sessionTimer = undefined;
    }
  };

  const evaluateDelivery = () => {
    if (deliveryEvaluated) {
      return;
    }

    deliveryEvaluated = true;
    const allOwnMessagesReceived =
      sentCount > 0 && receivedOwnCount === sentCount;

    messageDeliverySuccess.add(allOwnMessagesReceived);
    check({ sentCount, receivedOwnCount }, {
      'all own chat messages received': () => allOwnMessagesReceived,
    });
  };

  const startTraffic = () => {
    sendTimer = setInterval(() => {
      if (socket.readyState !== WebSocket.OPEN) {
        stopTimers();
        return;
      }

      sentCount += 1;
      const content = `${username}-message-${sentCount}`;
      sentAt.set(content, Date.now());

      socket.send(JSON.stringify({
        type: 'chat',
        sender: '',
        content,
      }));
    }, messageIntervalMs);

    sessionTimer = setTimeout(() => {
      clearInterval(sendTimer);
      sendTimer = undefined;

      // 마지막 메시지가 돌아올 시간을 기다린 후 결과를 판정한다.
      sessionTimer = setTimeout(() => {
        evaluateDelivery();

        if (socket.readyState === WebSocket.OPEN) {
          socket.close();
        }
      }, 1000);
    }, sessionHoldSeconds * 1000);
  };

  socket.addEventListener('open', () => {
    connected = true;
    connectionSuccess.add(true);

    socket.send(JSON.stringify({
      type: 'join',
      sender: username,
      content: '',
    }));

    // 입장 알림 문구가 변경되더라도 실제 채팅 부하는 계속 발생시킨다.
    // WebSocket은 전송 순서를 보장하므로 join 다음에 chat이 처리된다.
    startTraffic();

    joinTimer = setTimeout(() => {
      if (joinEvaluated) {
        return;
      }

      joinEvaluated = true;
      check(null, {
        'join broadcast received': () => false,
      });
    }, 5000);
  });

  socket.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);

    if (!joined && message.type === 'system' &&
        message.content === `${username}님이 입장했습니다.`) {
      joined = true;
      joinEvaluated = true;

      if (joinTimer !== undefined) {
        clearTimeout(joinTimer);
        joinTimer = undefined;
      }

      check(message, {
        'join broadcast received': () => true,
      });
      return;
    }

    if (message.type !== 'chat' || message.sender !== username) {
      return;
    }

    const startedAt = sentAt.get(message.content);
    if (startedAt === undefined) {
      return;
    }

    receivedOwnCount += 1;
    ownMessageLatency.add(Date.now() - startedAt);

    check(message, {
      'own chat broadcast received': (msg) => sentAt.has(msg.content),
      'message has database id': (msg) =>
        Number.isInteger(msg.id) && msg.id > 0,
    });

    sentAt.delete(message.content);
  });

  socket.addEventListener('error', () => {
    if (!connected) {
      connectionSuccess.add(false);
    }
  });

  socket.addEventListener('close', () => {
    if (connected && !joinEvaluated) {
      joinEvaluated = true;
      check(null, {
        'join broadcast received': () => false,
      });
    }

    evaluateDelivery();
    stopTimers();
  });
}
