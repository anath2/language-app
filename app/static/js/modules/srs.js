/**
 * SRS Module - Spaced Repetition System and vocabulary tracking
 */
const SRS = (() => {
	async function recordLookup(vocabItemId, segmentEl) {
		try {
			const data = await Api.postJson("/api/vocab/lookup", {
				vocab_item_id: vocabItemId,
			});
			if (data && segmentEl) {
				segmentEl.style.setProperty("--segment-opacity", data.opacity);
				if (data.is_struggling) {
					segmentEl.classList.add("struggling");
				} else {
					segmentEl.classList.remove("struggling");
				}
				const headword = segmentEl.textContent;
				const savedVocabMap = State.get("savedVocabMap");
				if (savedVocabMap.has(headword)) {
					savedVocabMap.get(headword).opacity = data.opacity;
					savedVocabMap.get(headword).isStruggling = data.is_struggling;
				}
			}
		} catch (e) {
			console.error("Failed to record lookup:", e);
		}
	}

	function updateAllSegmentInstances(headword, vocabItemId, opacity, isStruggling) {
		document.querySelectorAll(".segment").forEach((seg) => {
			if (seg.textContent === headword) {
				seg.classList.add("saved");
				seg.dataset.vocabItemId = vocabItemId;

				const originalColor =
					seg.style.getPropertyValue("--segment-color") ||
					seg.style.background ||
					Utils.getPastelColor(parseInt(seg.dataset.index, 10) || 0);
				seg.style.background = "";
				seg.style.setProperty("--segment-color", originalColor);
				seg.style.setProperty("--segment-opacity", opacity);

				if (isStruggling) {
					seg.classList.add("struggling");
				} else {
					seg.classList.remove("struggling");
				}
			}
		});
	}

	async function fetchAndApplySRSInfo() {
		const segments = document.querySelectorAll(".segment:not(.segment-pending)");
		const headwords = [
			...new Set(
				[...segments]
					.filter((s) => s.dataset.pinyin || s.dataset.english)
					.map((s) => s.textContent),
			),
		];
		if (headwords.length === 0) return;

		try {
			const params = new URLSearchParams();
			params.set("headwords", headwords.join(","));
			const res = await fetch(`/api/vocab/srs-info?${params.toString()}`);
			if (!res.ok) return;
			const data = await res.json();

			const savedVocabMap = State.get("savedVocabMap");
			savedVocabMap.clear();
			data.items.forEach((info) => {
				const opacity = info.status === "known" ? 0 : info.opacity;
				savedVocabMap.set(info.headword, {
					vocabItemId: info.vocab_item_id,
					opacity: opacity,
					isStruggling: info.is_struggling,
					status: info.status,
				});
			});

			segments.forEach((seg) => {
				const info = savedVocabMap.get(seg.textContent);
				if (info) {
					const originalColor =
						seg.style.background ||
						Utils.getPastelColor(parseInt(seg.dataset.index, 10) || 0);

					seg.classList.add("saved");
					seg.style.background = "";
					seg.style.setProperty("--segment-color", originalColor);
					seg.style.setProperty("--segment-opacity", info.opacity);
					seg.dataset.vocabItemId = info.vocabItemId;
					if (info.isStruggling) {
						seg.classList.add("struggling");
					}
				}
			});
		} catch (e) {
			console.error("Failed to fetch SRS info:", e);
		}
	}

	function applyPostStreamStyling(addSegmentInteraction) {
		const segments = document.querySelectorAll(".segment:not(.segment-pending)");
		const savedVocabMap = State.get("savedVocabMap");

		segments.forEach((seg) => {
			if (!seg.dataset.pinyin && !seg.dataset.english) {
				seg.style.background = "transparent";
				seg.style.cursor = "default";
				return;
			}

			const headword = seg.textContent;
			const info = savedVocabMap.get(headword);

			if (info) {
				const originalColor = Utils.getPastelColor(
					parseInt(seg.dataset.index, 10) || 0,
				);
				seg.classList.add("saved");
				seg.style.background = "";
				seg.style.setProperty("--segment-color", originalColor);
				seg.style.setProperty("--segment-opacity", info.opacity);
				seg.dataset.vocabItemId = info.vocabItemId;
				if (info.isStruggling) {
					seg.classList.add("struggling");
				}
			} else {
				seg.style.background = Utils.getPastelColor(
					parseInt(seg.dataset.index, 10) || 0,
				);
			}

			seg.style.cursor = "pointer";
			seg.classList.add(
				"transition-all",
				"duration-150",
				"hover:-translate-y-px",
				"hover:shadow-sm",
			);
			if (addSegmentInteraction) {
				addSegmentInteraction(seg);
			}
		});
	}

	async function updateDueCount() {
		try {
			const res = await fetch("/api/review/count");
			if (!res.ok) return;
			const data = await res.json();
			const badge = document.getElementById("review-badge");
			if (data.due_count > 0) {
				badge.textContent = data.due_count;
				badge.classList.remove("hidden");
			} else {
				badge.classList.add("hidden");
			}
		} catch (e) {
			console.error("Failed to fetch due count:", e);
		}
	}

	return {
		recordLookup,
		updateAllSegmentInstances,
		fetchAndApplySRSInfo,
		applyPostStreamStyling,
		updateDueCount,
	};
})();
