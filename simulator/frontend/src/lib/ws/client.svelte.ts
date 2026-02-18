import type { Envelope } from './messages';

export type MessageHandler = (envelope: Envelope) => void;

export class WSClient {
	private ws: WebSocket | null = null;
	private url: string;
	private handlers: MessageHandler[] = [];
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private _connected = $state(false);

	constructor(url: string) {
		this.url = url;
	}

	get connected(): boolean {
		return this._connected;
	}

	connect(): void {
		if (this.ws?.readyState === WebSocket.OPEN) return;

		try {
			this.ws = new WebSocket(this.url);

			this.ws.onopen = () => {
				this._connected = true;
				console.log('WebSocket connected');
			};

			this.ws.onclose = () => {
				this._connected = false;
				console.log('WebSocket disconnected, reconnecting...');
				this.scheduleReconnect();
			};

			this.ws.onerror = (err) => {
				console.error('WebSocket error:', err);
			};

			this.ws.onmessage = (event: MessageEvent) => {
				try {
					const envelope: Envelope = JSON.parse(event.data as string);
					for (const handler of this.handlers) {
						handler(envelope);
					}
				} catch (err) {
					console.error('Failed to parse WebSocket message:', err);
				}
			};
		} catch (err) {
			console.error('Failed to create WebSocket:', err);
			this.scheduleReconnect();
		}
	}

	disconnect(): void {
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		this.ws?.close();
		this.ws = null;
		this._connected = false;
	}

	onMessage(handler: MessageHandler): () => void {
		this.handlers.push(handler);
		return () => {
			this.handlers = this.handlers.filter((h) => h !== handler);
		};
	}

	send(type: string, payload?: unknown): void {
		if (this.ws?.readyState !== WebSocket.OPEN) {
			console.warn('WebSocket not connected, cannot send:', type);
			return;
		}

		const envelope: Envelope = { type };
		if (payload !== undefined) {
			envelope.payload = payload;
		}

		this.ws.send(JSON.stringify(envelope));
	}

	private scheduleReconnect(): void {
		if (this.reconnectTimer) return;
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			this.connect();
		}, 2000);
	}
}

function getWSUrl(): string {
	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
	return `${protocol}//${window.location.host}/ws`;
}

let clientInstance: WSClient | null = null;

export function getClient(): WSClient {
	if (!clientInstance) {
		clientInstance = new WSClient(getWSUrl());
	}
	return clientInstance;
}
