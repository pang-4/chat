import { sleep } from 'k6';
import websocketSession from './websocket-load.js';

const vus = Number(__ENV.VUS || 200);
const rampSeconds = Number(__ENV.RAMP_SECONDS || 120);
const holdSeconds = Number(__ENV.HOLD_SECONDS || 240);

export const options = {
  scenarios: {
    gradual_websocket_connections: {
      executor: 'per-vu-iterations',
      vus,
      iterations: 1,
      maxDuration: `${rampSeconds + holdSeconds + 30}s`,
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
  // HPA가 Metric을 수집하고 새 Pod를 생성할 시간을 확보하도록
  // VU별 WebSocket 연결 시작 시점을 전체 Ramp 구간에 균등하게 분산한다.
  const position = vus <= 1 ? 0 : (__VU - 1) / (vus - 1);
  sleep(position * rampSeconds);

  websocketSession();
}
