import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { WSClient } from '$lib/ws/client.svelte';

// Mock WebSocket
class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	url: string;
	readyState = MockWebSocket.CONNECTING;
	onopen: ((ev: Event) => void) | null = null;
	onclose: ((ev: CloseEvent) => void) | null = null;
	onerror: ((ev: Event) => void) | null = null;
	onmessage: ((ev: MessageEvent) => void) | null = null;

	sent: string[] = [];
	closed = false;

	constructor(url: string) {
		this.url = url;
		// Auto-open after microtask
		queueMicrotask(() => {
			this.readyState = MockWebSocket.OPEN;
			this.onopen?.(new Event('open'));
		});
	}

	send(data: string) {
		this.sent.push(data);
	}

	close() {
		if (this.closed) return;
		this.closed = true;
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.(new CloseEvent('close'));
	}

	// Test helpers
	simulateMessage(data: string) {
		this.onmessage?.(new MessageEvent('message', { data }));
	}

	simulateError() {
		this.onerror?.(new Event('error'));
	}
}

let instances: MockWebSocket[] = [];

beforeEach(() => {
	instances = [];
	vi.stubGlobal(
		'WebSocket',
		class extends MockWebSocket {
			constructor(url: string) {
				super(url);
				instances.push(this);
			}
		}
	);
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
	vi.restoreAllMocks();
});

function flushMicrotasks() {
	return new Promise<void>((resolve) => queueMicrotask(resolve));
}

describe('WSClient', () => {
	it('connects and becomes connected', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		client.connect();
		await flushMicrotasks();

		expect(client.connected).toBe(true);
		expect(instances).toHaveLength(1);
		expect(instances[0].url).toBe('ws://localhost:8080/ws');
	});

	it('does not reconnect if already open', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		client.connect();
		await flushMicrotasks();

		client.connect(); // second call
		expect(instances).toHaveLength(1);
	});

	it('disconnects cleanly', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		client.connect();
		await flushMicrotasks();

		client.disconnect();
		expect(client.connected).toBe(false);
		expect(instances[0].closed).toBe(true);
	});

	it('sends messages when connected', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		client.connect();
		await flushMicrotasks();

		client.send('sim:start');
		expect(instances[0].sent).toHaveLength(1);

		const sent = JSON.parse(instances[0].sent[0]);
		expect(sent.type).toBe('sim:start');
		expect(sent.payload).toBeUndefined();
	});

	it('sends messages with payload', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		client.connect();
		await flushMicrotasks();

		client.send('sim:set_speed', { speed: 7200 });
		const sent = JSON.parse(instances[0].sent[0]);
		expect(sent.type).toBe('sim:set_speed');
		expect(sent.payload).toEqual({ speed: 7200 });
	});

	it('warns when sending while disconnected', async () => {
		const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
		const client = new WSClient('ws://localhost:8080/ws');

		client.send('sim:start');
		expect(warnSpy).toHaveBeenCalledWith(
			'WebSocket not connected, cannot send:',
			'sim:start'
		);
	});

	it('dispatches messages to handlers', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		const received: any[] = [];

		client.onMessage((envelope) => {
			received.push(envelope);
		});

		client.connect();
		await flushMicrotasks();

		instances[0].simulateMessage(JSON.stringify({ type: 'sim:state', payload: { time: 'T', speed: 1, running: true } }));

		expect(received).toHaveLength(1);
		expect(received[0].type).toBe('sim:state');
	});

	it('dispatches to multiple handlers', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		const r1: any[] = [];
		const r2: any[] = [];

		client.onMessage((e) => r1.push(e));
		client.onMessage((e) => r2.push(e));

		client.connect();
		await flushMicrotasks();

		instances[0].simulateMessage(JSON.stringify({ type: 'test' }));

		expect(r1).toHaveLength(1);
		expect(r2).toHaveLength(1);
	});

	it('unsubscribes handler', async () => {
		const client = new WSClient('ws://localhost:8080/ws');
		const received: any[] = [];

		const unsub = client.onMessage((e) => received.push(e));

		client.connect();
		await flushMicrotasks();

		instances[0].simulateMessage(JSON.stringify({ type: 'msg1' }));
		unsub();
		instances[0].simulateMessage(JSON.stringify({ type: 'msg2' }));

		expect(received).toHaveLength(1);
		expect(received[0].type).toBe('msg1');
	});

	it('handles invalid JSON gracefully', async () => {
		const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const client = new WSClient('ws://localhost:8080/ws');
		const received: any[] = [];

		client.onMessage((e) => received.push(e));
		client.connect();
		await flushMicrotasks();

		instances[0].simulateMessage('not json');

		expect(received).toHaveLength(0);
		expect(errorSpy).toHaveBeenCalled();
	});

	it('schedules reconnect on close', async () => {
		vi.spyOn(console, 'log').mockImplementation(() => {});
		const client = new WSClient('ws://localhost:8080/ws');
		client.connect();
		await flushMicrotasks();

		// Simulate disconnect
		instances[0].readyState = MockWebSocket.CLOSED;
		instances[0].onclose?.(new CloseEvent('close'));

		expect(client.connected).toBe(false);

		// Advance timers to trigger reconnect (2s)
		vi.advanceTimersByTime(2000);
		await flushMicrotasks();

		// Should have created a new WebSocket
		expect(instances).toHaveLength(2);
	});

	it('disconnect cancels pending reconnect', async () => {
		vi.spyOn(console, 'log').mockImplementation(() => {});
		const client = new WSClient('ws://localhost:8080/ws');
		client.connect();
		await flushMicrotasks();

		// Simulate server-initiated close → schedules reconnect.
		// Mark mock as closed so that disconnect()'s ws.close() doesn't re-trigger onclose.
		instances[0].closed = true;
		instances[0].readyState = MockWebSocket.CLOSED;
		instances[0].onclose?.(new CloseEvent('close'));

		// Explicitly disconnect → should cancel reconnect
		client.disconnect();

		vi.advanceTimersByTime(3000);
		await flushMicrotasks();

		// No reconnect should have happened
		expect(instances).toHaveLength(1);
	});
});
