/**
 * Api Module - API helper functions
 */
const Api = (() => {
	async function postJson(path, payload) {
		const res = await fetch(path, {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(payload),
		});
		if (!res.ok) {
			let detail = "";
			try {
				detail = await res.text();
			} catch (_e) {}
			throw new Error(`Request failed (${res.status}): ${detail}`);
		}
		return await res.json();
	}

	async function ensureSavedText() {
		const currentTextId = State.get("currentTextId");
		const currentRawText = State.get("currentRawText");

		if (currentTextId || !currentRawText) return currentTextId;

		const data = await postJson("/api/texts", {
			raw_text: currentRawText,
			source_type: "text",
			metadata: {},
		});
		State.set("currentTextId", data.id);
		return data.id;
	}

	async function logEvent(eventType, payload = {}) {
		try {
			await postJson("/api/events", {
				event_type: eventType,
				text_id: State.get("currentTextId"),
				payload,
			});
		} catch (_e) {
			// swallow
		}
	}

	return {
		postJson,
		ensureSavedText,
		logEvent,
	};
})();
