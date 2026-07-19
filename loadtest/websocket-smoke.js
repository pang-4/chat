import { WebSocket } from 'k6/websockets';
import { check } from 'k6';

const wsURL = __ENV.WS_URL || 'ws://localhost:8080/ws';

export const options = {
  vus: 1,
  iterations: 1,

  thresholds: {
    checks: ['rate>0.99'],
  },
};

export default function () {
  const socket = new WebSocket(wsURL);

  const username = `k6-user-${__VU}`;
  const chatContent = `Hello from ${username}`;

  socket.addEventListener('open', () => {
    const joinMessage = {
      type: 'join',
      sender: username,
      content: '',
    };

    socket.send(JSON.stringify(joinMessage));
  });

  socket.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);

    if (message.type === 'system') {
      const joinSucceeded = check(message, {
        'join broadcast received': (msg) =>
          msg.content === `${username}님이 입장했습니다.`,
      });

      if (!joinSucceeded) {
        socket.close();
        return;
      }

      const chatMessage = {
        type: 'chat',
        sender: '',
        content: chatContent,
      };

      socket.send(JSON.stringify(chatMessage));
      return;
    }

    if (message.type === 'chat') {
      check(message, {
        'chat broadcast received': (msg) =>
          msg.sender === username && msg.content === chatContent,

        'message has database id': (msg) =>
          Number.isInteger(msg.id) && msg.id > 0,
      });

      socket.close();
    }
  });
}