/**
 * State Module - Centralized state management
 */
const State = (() => {
	// Internal state store
	const state = {
		// Persistence state
		currentTextId: null,
		currentRawText: "",

		// Translation state
		translationResults: [],
		isClickActive: false,
		activeSegment: null,
		lastTooltipData: null,

		// SRS state
		savedVocabMap: new Map(),
		reviewQueue: [],
		reviewIndex: 0,
		reviewAnswered: false,

		// Job queue state
		jobQueue: [],
		currentJobId: null,
		isExpandedView: false,
	};

	// Subscribers for reactive updates
	const subscribers = new Map();

	function get(key) {
		if (!(key in state)) {
			console.warn(`State key "${key}" does not exist`);
			return undefined;
		}
		return state[key];
	}

	function set(key, value) {
		if (!(key in state)) {
			console.warn(`State key "${key}" does not exist`);
			return;
		}
		const oldValue = state[key];
		state[key] = value;

		// Notify subscribers
		if (subscribers.has(key)) {
			subscribers.get(key).forEach((callback) => callback(value, oldValue));
		}
	}

	function subscribe(key, callback) {
		if (!subscribers.has(key)) {
			subscribers.set(key, new Set());
		}
		subscribers.get(key).add(callback);

		// Return unsubscribe function
		return () => {
			subscribers.get(key).delete(callback);
		};
	}

	// Batch update multiple state values
	function update(updates) {
		Object.entries(updates).forEach(([key, value]) => {
			set(key, value);
		});
	}

	// Reset specific state keys to initial values
	function reset(keys) {
		const initialValues = {
			currentTextId: null,
			currentRawText: "",
			translationResults: [],
			isClickActive: false,
			activeSegment: null,
			lastTooltipData: null,
			savedVocabMap: new Map(),
			reviewQueue: [],
			reviewIndex: 0,
			reviewAnswered: false,
			jobQueue: [],
			currentJobId: null,
			isExpandedView: false,
		};

		keys.forEach((key) => {
			if (key in initialValues) {
				set(key, initialValues[key]);
			}
		});
	}

	return {
		get,
		set,
		subscribe,
		update,
		reset,
	};
})();
