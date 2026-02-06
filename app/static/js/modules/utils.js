/**
 * Utils Module - Shared utility functions
 */
const Utils = (() => {
	// Pastel colors for segment highlighting
	const pastelColors = [
		"#FFB3BA", // pink
		"#BAFFC9", // mint
		"#BAE1FF", // sky
		"#FFFFBA", // lemon
		"#FFD9BA", // peach
		"#E0BBE4", // lavender
		"#C9F0FF", // ice
		"#FFDAB3", // apricot
	];

	function getPastelColor(index) {
		return pastelColors[index % pastelColors.length];
	}

	function escapeHtml(text) {
		const div = document.createElement("div");
		div.textContent = text;
		return div.innerHTML;
	}

	function formatTimeAgo(isoString) {
		const date = new Date(isoString);
		const now = new Date();
		const seconds = Math.floor((now - date) / 1000);

		if (seconds < 60) return "Just now";
		if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
		if (seconds < 86400) return `${Math.floor(seconds / 3600)} hr ago`;
		return date.toLocaleDateString();
	}

	return {
		getPastelColor,
		escapeHtml,
		formatTimeAgo,
	};
})();
