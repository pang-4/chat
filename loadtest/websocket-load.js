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

export default function () {
  // VU를 WebSocket 주소에 순서대로 분배한다.
  // VU 1 → 첫 번째 주소, VU 2 → 두 번째 주소, VU 3 → 세 번째 주소,
  // VU 4 → 다시 첫 번째 주소로 연결된다.
  const wsURL = wsURLs[(__VU - 1) % wsURLs.length];
  const socket = new WebSocket(wsURL);
  const username = `k6-user-${__VU}-${__ITER}`;

  let connected = false;
  let joined = false;
  let sentCount = 0;
  let receivedOwnCount = 0;
  const sentAt = new Map();

  socket.addEventListener('open', () => {
    connected = true;
    connectionSuccess.add(true);

    socket.send(JSON.stringify({
      type: 'join',
      sender: username,
      content: '',
    }));

    setTimeout(() => {
      if (joined) {
        return;
      }

      messageDeliverySuccess.add(false);
      check(null, {
        'join broadcast received': () => false,
      });
      socket.close();
    }, 5000);
  });

  socket.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);

    if (!joined && message.type === 'system' &&
        message.content === `${username}님이 입장했습니다.`) {
      joined = true;

      check(message, {
        'join broadcast received': () => true,
      });

      const sendTimer = setInterval(() => {
        sentCount += 1;
        const content = `${username}-message-${sentCount}`;
        sentAt.set(content, Date.now());

        socket.send(JSON.stringify({
          type: 'chat',
          sender: '',
          content,
        }));
      }, messageIntervalMs);

      setTimeout(() => {
        clearInterval(sendTimer);

        // 마지막 메시지가 돌아올 시간을 조금 기다린 후 결과를 판정한다.
        setTimeout(() => {
          const allOwnMessagesReceived =
            sentCount > 0 && receivedOwnCount === sentCount;

          messageDeliverySuccess.add(allOwnMessagesReceived);
          check({ sentCount, receivedOwnCount }, {
            'all own chat messages received': () => allOwnMessagesReceived,
          });

          socket.close();
        }, 1000);
      }, holdSeconds * 1000);

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
}
