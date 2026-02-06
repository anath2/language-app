/**
 * SegmentInteractions Module - Tooltip and click handling for segments
 */
const SegmentInteractions = (() => {
	function showTooltip(seg) {
		const tooltip = document.getElementById("word-tooltip");
		const tooltipPinyin = document.getElementById("tooltip-pinyin");
		const tooltipEnglish = document.getElementById("tooltip-english");

		document.querySelectorAll(".segment").forEach((s) => {
			s.classList.remove("border-gray-800", "shadow-lg");
		});

		tooltipPinyin.textContent = seg.dataset.pinyin;
		tooltipEnglish.textContent = seg.dataset.english;
		State.set("lastTooltipData", {
			headword: seg.textContent,
			pinyin: seg.dataset.pinyin || "",
			english: seg.dataset.english || "",
		});
		const saveStatus = document.getElementById("save-word-status");
		if (saveStatus) saveStatus.textContent = "";

		const saveBtn = document.getElementById("save-word-btn");
		const markKnownBtn = document.getElementById("mark-known-btn");
		const resumeLearningBtn = document.getElementById("resume-learning-btn");

		if (!seg.dataset.pinyin && !seg.dataset.english) {
			saveBtn.classList.add("hidden");
			markKnownBtn.classList.add("hidden");
			resumeLearningBtn.classList.add("hidden");
		} else {
			const savedVocabMap = State.get("savedVocabMap");
			const info = savedVocabMap.get(seg.textContent);

			if (!info) {
				saveBtn.classList.remove("hidden");
				markKnownBtn.classList.add("hidden");
				resumeLearningBtn.classList.add("hidden");
			} else if (info.status === "learning") {
				saveBtn.classList.add("hidden");
				markKnownBtn.classList.remove("hidden");
				resumeLearningBtn.classList.add("hidden");
			} else if (info.status === "known") {
				saveBtn.classList.add("hidden");
				markKnownBtn.classList.add("hidden");
				resumeLearningBtn.classList.remove("hidden");
			} else {
				saveBtn.classList.remove("hidden");
				markKnownBtn.classList.add("hidden");
				resumeLearningBtn.classList.add("hidden");
			}
		}

		const segRect = seg.getBoundingClientRect();
		const containerRect = seg.closest(".relative").getBoundingClientRect();

		tooltip.classList.remove("hidden");

		const left =
			segRect.left -
			containerRect.left +
			segRect.width / 2 -
			tooltip.offsetWidth / 2;
		const top = segRect.top - containerRect.top - tooltip.offsetHeight - 4;

		tooltip.style.left = `${Math.max(0, left)}px`;
		tooltip.style.top = `${top}px`;

		seg.classList.add("border-gray-800", "shadow-lg");
	}

	function hideTooltip() {
		const tooltip = document.getElementById("word-tooltip");
		if (tooltip) {
			tooltip.classList.add("hidden");
		}
		document.querySelectorAll(".segment").forEach((s) => {
			s.classList.remove("border-gray-800", "shadow-lg");
		});
	}

	function addSegmentInteraction(segment) {
		// Prevent duplicate event listeners
		if (segment.dataset.hasInteraction === "true") return;
		segment.dataset.hasInteraction = "true";

		segment.addEventListener("mouseenter", () => {
			if (!State.get("isClickActive")) {
				showTooltip(segment);
			}
		});

		segment.addEventListener("mouseleave", () => {
			if (!State.get("isClickActive")) {
				hideTooltip();
			}
		});

		segment.addEventListener("click", (e) => {
			e.stopPropagation();
			const isClickActive = State.get("isClickActive");
			const activeSegment = State.get("activeSegment");

			if (isClickActive && activeSegment === segment) {
				State.set("isClickActive", false);
				State.set("activeSegment", null);
				hideTooltip();
			} else {
				State.set("isClickActive", true);
				State.set("activeSegment", segment);
				showTooltip(segment);
				Api.logEvent("tap", { headword: segment.textContent });

				if (segment.dataset.vocabItemId) {
					SRS.recordLookup(segment.dataset.vocabItemId, segment);
				}
			}
		});
	}

	function setupEditButton() {
		const editBtn = document.getElementById("edit-segments-btn");
		if (editBtn) {
			editBtn.addEventListener("click", (e) => {
				e.stopPropagation();
				SegmentEditor.enterGlobalEditMode();
			});
		}
	}

	function setupGlobalClickDismiss() {
		document.addEventListener("click", (e) => {
			const tooltip = document.getElementById("word-tooltip");
			const clickedInTooltip = tooltip?.contains(e.target);
			if (State.get("isClickActive") && !clickedInTooltip && !e.target.closest(".segment")) {
				State.set("isClickActive", false);
				State.set("activeSegment", null);
				if (tooltip) {
					tooltip.classList.add("hidden");
				}
				document.querySelectorAll(".segment").forEach((s) => {
					s.classList.remove("border-gray-800", "shadow-lg");
				});
			}
		});
	}

	function setupVocabButtons() {
		// Save button inside tooltip
		document.addEventListener("click", async (e) => {
			if (e.target && e.target.id === "save-word-btn") {
				e.stopPropagation();
				const statusEl = document.getElementById("save-word-status");
				try {
					await Api.ensureSavedText();
					const lastTooltipData = State.get("lastTooltipData");
					if (!lastTooltipData || !lastTooltipData.headword) return;
					const result = await Api.postJson("/api/vocab/save", {
						headword: lastTooltipData.headword,
						pinyin: lastTooltipData.pinyin,
						english: lastTooltipData.english,
						text_id: State.get("currentTextId"),
						snippet: State.get("currentRawText"),
						status: "learning",
					});
					if (statusEl) statusEl.textContent = "Saved";
					Api.logEvent("save_vocab", { headword: lastTooltipData.headword });

					if (result.vocab_item_id) {
						const savedVocabMap = State.get("savedVocabMap");
						savedVocabMap.set(lastTooltipData.headword, {
							vocabItemId: result.vocab_item_id,
							opacity: 1.0,
							isStruggling: false,
							status: "learning",
						});

						SRS.updateAllSegmentInstances(
							lastTooltipData.headword,
							result.vocab_item_id,
							1.0,
							false,
						);

						document.getElementById("save-word-btn").classList.add("hidden");
						document
							.getElementById("mark-known-btn")
							.classList.remove("hidden");
					}

					SRS.updateDueCount();
				} catch (_err) {
					if (statusEl) statusEl.textContent = "Error";
				}
			}
		});

		// Mark as Known button handler
		document.addEventListener("click", async (e) => {
			if (e.target && e.target.id === "mark-known-btn") {
				e.stopPropagation();
				const statusEl = document.getElementById("save-word-status");
				try {
					const activeSegment = State.get("activeSegment");
					if (!activeSegment || !activeSegment.dataset.vocabItemId) return;
					const vocabItemId = activeSegment.dataset.vocabItemId;
					await Api.postJson("/api/vocab/status", {
						vocab_item_id: vocabItemId,
						status: "known",
					});
					if (statusEl) statusEl.textContent = "Marked known";
					const headword = activeSegment.textContent;
					Api.logEvent("mark_known", { headword: headword });

					const savedVocabMap = State.get("savedVocabMap");
					if (savedVocabMap.has(headword)) {
						const info = savedVocabMap.get(headword);
						info.status = "known";
						info.opacity = 0;
					}

					SRS.updateAllSegmentInstances(headword, vocabItemId, 0, false);

					document.getElementById("mark-known-btn").classList.add("hidden");
					document
						.getElementById("resume-learning-btn")
						.classList.remove("hidden");

					SRS.updateDueCount();
				} catch (_err) {
					if (statusEl) statusEl.textContent = "Error";
				}
			}
		});

		// Resume Learning button handler
		document.addEventListener("click", async (e) => {
			if (e.target && e.target.id === "resume-learning-btn") {
				e.stopPropagation();
				const statusEl = document.getElementById("save-word-status");
				try {
					const activeSegment = State.get("activeSegment");
					if (!activeSegment || !activeSegment.dataset.vocabItemId) return;
					const vocabItemId = activeSegment.dataset.vocabItemId;
					await Api.postJson("/api/vocab/status", {
						vocab_item_id: vocabItemId,
						status: "learning",
					});
					if (statusEl) statusEl.textContent = "Resumed";
					const headword = activeSegment.textContent;
					Api.logEvent("resume_learning", { headword: headword });

					const savedVocabMap = State.get("savedVocabMap");
					if (savedVocabMap.has(headword)) {
						const info = savedVocabMap.get(headword);
						info.status = "learning";
						info.opacity = 1.0;
					}

					SRS.updateAllSegmentInstances(headword, vocabItemId, 1.0, false);

					document
						.getElementById("resume-learning-btn")
						.classList.add("hidden");
					document.getElementById("mark-known-btn").classList.remove("hidden");

					SRS.updateDueCount();
				} catch (_err) {
					if (statusEl) statusEl.textContent = "Error";
				}
			}
		});
	}

	function setupEditButtonHandler() {
		// Global Edit button handler (from header)
		document.addEventListener("click", (e) => {
			if (
				e.target &&
				(e.target.id === "edit-segments-btn" ||
					e.target.closest("#edit-segments-btn"))
			) {
				e.stopPropagation();
				SegmentEditor.enterGlobalEditMode();
			}
		});
	}

	function init() {
		setupGlobalClickDismiss();
		setupVocabButtons();
		setupEditButtonHandler();
	}

	return {
		showTooltip,
		hideTooltip,
		addSegmentInteraction,
		setupEditButton,
		init,
	};
})();
