/**
 * Segment Editor - Global Edit Mode for Split/Join Operations
 * Provides a global edit mode where all segments can be split/joined
 */
// biome-ignore lint/correctness/noUnusedVariables: Used globally via window.SegmentEditor
const SegmentEditor = (() => {
	// State
	let isEditModeActive = false;
	const pendingSegments = new Set(); // DOM element references for segments needing translation
	let originalSegments = []; // Snapshot for cancel

	// Dependencies (set by init)
	let getPastelColor = null;
	let addSegmentInteraction = null;
	let getTranslationResults = null;
	let setTranslationResults = null;
	let getJobId = null; // Optional: returns current job ID for persistence

	// ========================================
	// Initialization
	// ========================================

	function init(deps) {
		getPastelColor = deps.getPastelColor;
		addSegmentInteraction = deps.addSegmentInteraction;
		getTranslationResults = deps.getTranslationResults;
		setTranslationResults = deps.setTranslationResults;
		getJobId = deps.getJobId || null; // Optional dependency

		// Delegate events for split/join functionality
		document.addEventListener("click", handleEditClick, true);
		document.addEventListener("keydown", handleEditKeydown);
	}

	// ========================================
	// Global Edit Mode
	// ========================================

	function enterGlobalEditMode() {
		if (isEditModeActive) return;
		isEditModeActive = true;

		// Capture original state for cancel
		captureOriginalState();

		// Hide the Edit button
		const editBtn = document.getElementById("edit-segments-btn");
		if (editBtn) editBtn.style.display = "none";

		// Add editing class to results container
		const resultsContainer = document.getElementById("results");
		if (resultsContainer) {
			resultsContainer.classList.add("edit-mode-active");
		}

		// Transform all segments to show split points
		document.querySelectorAll(".segment").forEach((seg) => {
			addSplitPointsToSegment(seg);
		});

		// Add join indicators between all adjacent segments
		addAllJoinIndicators();

		// Show control bar
		showEditBar();
	}

	function exitGlobalEditMode() {
		if (!isEditModeActive) return;
		isEditModeActive = false;

		// Remove split points from all segments
		document.querySelectorAll(".segment").forEach((seg) => {
			removeSplitPointsFromSegment(seg);
		});

		// Remove all join indicators
		document.querySelectorAll(".join-indicator").forEach((el) => {
			el.remove();
		});

		// Remove editing class from results container
		const resultsContainer = document.getElementById("results");
		if (resultsContainer) {
			resultsContainer.classList.remove("edit-mode-active");
		}

		// Show the Edit button again
		const editBtn = document.getElementById("edit-segments-btn");
		if (editBtn) editBtn.style.display = "";

		// Hide control bar
		hideEditBar();
	}

	function addSplitPointsToSegment(segment) {
		const text = segment.textContent.trim();
		if (text.length <= 1) return; // Can't split single character

		const originalBg =
			segment.style.background ||
			getPastelColor(parseInt(segment.dataset.index, 10) || 0);

		// Store original state
		segment.dataset.originalText = text;
		segment.dataset.originalBg = originalBg;

		// Build character wrappers with clickable split points
		let html = "";
		for (let i = 0; i < text.length; i++) {
			html += `<span class="char-wrapper" data-char-index="${i}" style="background: ${originalBg}; border-radius: 3px; padding: 0.25rem 0.15rem;">${text[i]}`;
			if (i < text.length - 1) {
				html += `<span class="split-point" data-split-after="${i}" title="Split here"></span>`;
			}
			html += `</span>`;
		}

		segment.innerHTML = html;
		segment.classList.add("editing");
	}

	function removeSplitPointsFromSegment(segment) {
		if (!segment.classList.contains("editing")) return;

		const originalText = segment.dataset.originalText;
		const originalBg = segment.dataset.originalBg;

		if (originalText) {
			segment.innerHTML = originalText;
			segment.style.background = originalBg;
		}
		segment.classList.remove("editing");
	}

	function addAllJoinIndicators() {
		document.querySelectorAll(".paragraph").forEach((para) => {
			const segments = para.querySelectorAll(".segment");
			segments.forEach((seg, i) => {
				if (i < segments.length - 1) {
					const nextSeg = segments[i + 1];
					// Add join indicator between this segment and next
					const joinIndicator = document.createElement("span");
					joinIndicator.className = "join-indicator visible";
					joinIndicator.innerHTML = "⊕";
					joinIndicator.dataset.leftIndex = seg.dataset.index;
					joinIndicator.dataset.rightIndex = nextSeg.dataset.index;
					joinIndicator.title = "Join segments";
					seg.parentNode.insertBefore(joinIndicator, nextSeg);
				}
			});
		});
	}

	function refreshEditModeUI() {
		if (!isEditModeActive) return;

		// Remove old join indicators
		document.querySelectorAll(".join-indicator").forEach((el) => {
			el.remove();
		});

		// Re-add split points to new segments
		document.querySelectorAll(".segment").forEach((seg) => {
			if (!seg.classList.contains("editing")) {
				addSplitPointsToSegment(seg);
			}
		});

		// Re-add join indicators
		addAllJoinIndicators();
	}

	// ========================================
	// Edit Bar (Save/Cancel Controls)
	// ========================================

	function showEditBar() {
		if (document.querySelector(".segment-edit-bar")) return;

		const resultsContainer = document.getElementById("results");
		if (!resultsContainer) return;

		const bar = document.createElement("div");
		bar.className = "segment-edit-bar";
		bar.innerHTML = `
            <span class="edit-bar-status">
                <span class="pending-count">${pendingSegments.size}</span> changes
            </span>
            <div class="edit-bar-actions">
                <button class="btn-cancel" type="button">Cancel</button>
                <button class="btn-save" type="button">Save Changes</button>
            </div>
        `;

		// Add event listeners
		bar.querySelector(".btn-cancel").addEventListener("click", cancelEdits);
		bar.querySelector(".btn-save").addEventListener("click", saveEdits);

		resultsContainer.appendChild(bar);
	}

	function hideEditBar() {
		document.querySelector(".segment-edit-bar")?.remove();
	}

	function updateEditBar() {
		const countEl = document.querySelector(".pending-count");
		if (countEl) {
			countEl.textContent = pendingSegments.size;
		}
	}

	// ========================================
	// Original State Management
	// ========================================

	function captureOriginalState() {
		const results = getTranslationResults();
		originalSegments = results.map((r) => ({
			segment: r.segment,
			pinyin: r.pinyin,
			english: r.english,
		}));
	}

	function cancelEdits() {
		// Restore original segments
		setTranslationResults([...originalSegments]);

		// Re-render segments from original
		rebuildSegmentDisplay();

		pendingSegments.clear();
		exitGlobalEditMode();
	}

	async function saveEdits() {
		if (pendingSegments.size === 0) {
			exitGlobalEditMode();
			return;
		}

		const saveBtn = document.querySelector(".btn-save");
		if (saveBtn) {
			saveBtn.disabled = true;
			saveBtn.textContent = "Saving...";
		}

		// Collect segments by DOM reference, filter out any removed elements, sort by index
		const segments = [...pendingSegments].filter((seg) =>
			document.body.contains(seg),
		);
		segments.sort(
			(a, b) => parseInt(a.dataset.index, 10) - parseInt(b.dataset.index, 10),
		);
		const segmentTexts = segments
			.map((seg) => seg.dataset.originalText || seg.textContent.trim())
			.filter((t) => t);

		try {
			// Build request body with optional job_id for persistence
			const requestBody = {
				segments: segmentTexts,
				context: window.App?.currentRawText || null,
			};

			// Include job_id and paragraph_idx if available (for persisting edits)
			const currentJobId = getJobId
				? getJobId()
				: window.App?.currentJobId || null;
			if (currentJobId) {
				// Get paragraph index from first pending segment
				const firstSeg = segments[0];
				const paragraphIdx = firstSeg
					? parseInt(firstSeg.dataset.paragraph, 10) || 0
					: 0;
				requestBody.job_id = currentJobId;
				requestBody.paragraph_idx = paragraphIdx;
			}

			const response = await fetch("/api/segments/translate-batch", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(requestBody),
			});

			if (!response.ok) {
				throw new Error("Translation failed");
			}

			const { translations } = await response.json();

			// Update translation results using current segment indices
			const translationResults = getTranslationResults();
			segments.forEach((seg, i) => {
				const idx = parseInt(seg.dataset.index, 10);
				if (translations[i] && translationResults[idx]) {
					translationResults[idx] = translations[i];
				}
			});
			setTranslationResults(translationResults);

			// Update DOM segments with new translations BEFORE exiting edit mode
			// This ensures originalText gets the real translation, not placeholder
			segments.forEach((seg, i) => {
				if (seg && translations[i]) {
					seg.dataset.pinyin = translations[i].pinyin;
					seg.dataset.english = translations[i].english;
					// Update originalText so exitGlobalEditMode restores correct text
					if (seg.dataset.originalText) {
						seg.dataset.originalText = translations[i].segment;
					}
				}
			});

			pendingSegments.clear();
			exitGlobalEditMode();
			rebuildSegmentDisplay();
			rebuildTranslationTable();

			console.log("Save completed:", translations);
		} catch (error) {
			console.error("Save failed:", error);
			if (saveBtn) {
				saveBtn.disabled = false;
				saveBtn.textContent = "Save Changes";
			}
			alert("Failed to translate segments. Please try again.");
		}
	}

	// ========================================
	// Event Handlers
	// ========================================

	function handleEditClick(e) {
		// Handle split point click
		const splitPoint = e.target.closest(".split-point");
		if (splitPoint && isEditModeActive) {
			e.stopPropagation();
			const segment = splitPoint.closest(".segment");
			const splitAfter = parseInt(splitPoint.dataset.splitAfter, 10);
			performSplit(segment, splitAfter);
			return;
		}

		// Handle join indicator click
		const joinIndicator = e.target.closest(".join-indicator");
		if (joinIndicator && isEditModeActive) {
			e.stopPropagation();
			const leftIndex = parseInt(joinIndicator.dataset.leftIndex, 10);
			const rightIndex = parseInt(joinIndicator.dataset.rightIndex, 10);
			performJoin(leftIndex, rightIndex);
			return;
		}
	}

	function handleEditKeydown(e) {
		// Escape exits edit mode
		if (e.key === "Escape" && isEditModeActive) {
			cancelEdits();
		}
	}

	// ========================================
	// Local Split/Join (no API call, placeholders)
	// ========================================

	function splitSegmentLocal(segmentText, splitAfter) {
		const leftText = segmentText.substring(0, splitAfter + 1);
		const rightText = segmentText.substring(splitAfter + 1);

		return {
			segments: [
				{ text: leftText, pinyin: "...", english: `[${leftText}]` },
				{ text: rightText, pinyin: "...", english: `[${rightText}]` },
			],
		};
	}

	function joinSegmentsLocal(leftText, rightText) {
		const mergedText = leftText + rightText;
		return {
			segment: { text: mergedText, pinyin: "...", english: `[${mergedText}]` },
		};
	}

	// ========================================
	// Split/Join Operations
	// ========================================

	function performSplit(segment, splitAfter) {
		const segmentIndex = parseInt(segment.dataset.index, 10);
		const paragraphIndex = parseInt(segment.dataset.paragraph, 10);
		const originalText =
			segment.dataset.originalText || segment.textContent.trim();

		// Remove split points before splitting
		removeSplitPointsFromSegment(segment);

		const targetSegment = document.querySelector(
			`.segment[data-index="${segmentIndex}"]`,
		);
		if (!targetSegment) return;

		// Local split (no API call)
		const result = splitSegmentLocal(originalText, splitAfter);
		updateSegmentsAfterSplit(
			targetSegment,
			result.segments,
			paragraphIndex,
			true,
		);

		// Refresh edit mode UI for new segments
		refreshEditModeUI();

		console.log("Split completed (pending translation):", result);
	}

	function performJoin(leftIndex, rightIndex) {
		let leftSegment = document.querySelector(
			`.segment[data-index="${leftIndex}"]`,
		);
		let rightSegment = document.querySelector(
			`.segment[data-index="${rightIndex}"]`,
		);

		if (!leftSegment || !rightSegment) return;

		// Get text from original or current
		const leftText =
			leftSegment.dataset.originalText || leftSegment.textContent.trim();
		const rightText =
			rightSegment.dataset.originalText || rightSegment.textContent.trim();
		const paragraphIndex = parseInt(leftSegment.dataset.paragraph, 10);

		// Remove split points before joining
		removeSplitPointsFromSegment(leftSegment);
		removeSplitPointsFromSegment(rightSegment);

		// Re-query after removing split points
		leftSegment = document.querySelector(`.segment[data-index="${leftIndex}"]`);
		rightSegment = document.querySelector(
			`.segment[data-index="${rightIndex}"]`,
		);

		if (!leftSegment || !rightSegment) return;

		// Local join (no API call)
		const result = joinSegmentsLocal(leftText, rightText);
		updateSegmentsAfterJoin(
			leftSegment,
			rightSegment,
			result.segment,
			paragraphIndex,
			true,
		);

		// Refresh edit mode UI
		refreshEditModeUI();

		console.log("Join completed (pending translation):", result);
	}

	// ========================================
	// UI Update Functions
	// ========================================

	function updateSegmentsAfterSplit(
		originalSegment,
		newSegments,
		paragraphIndex,
		isPending = false,
	) {
		const segmentIndex = parseInt(originalSegment.dataset.index, 10);
		const paragraph = originalSegment.parentNode;

		// Remove original segment from pending set if it was there
		pendingSegments.delete(originalSegment);

		const fragment = document.createDocumentFragment();

		newSegments.forEach((seg, i) => {
			const newIndex = segmentIndex + i;
			const span = document.createElement("span");
			span.className =
				"segment inline-block px-2 py-1 rounded border-2 border-transparent";
			span.style.fontFamily = "var(--font-chinese)";
			span.style.fontSize = "var(--text-chinese)";
			span.style.color = "var(--text-primary)";
			span.dataset.index = newIndex;
			span.dataset.paragraph = paragraphIndex;
			span.dataset.pinyin = seg.pinyin;
			span.dataset.english = seg.english;
			span.textContent = seg.text;

			span.style.background = getPastelColor(newIndex);
			span.style.cursor = "pointer";
			span.classList.add(
				"transition-all",
				"duration-150",
				"hover:-translate-y-px",
				"hover:shadow-sm",
			);

			if (isPending) {
				span.classList.add("segment-pending");
				pendingSegments.add(span);
			}

			addSegmentInteraction(span);
			fragment.appendChild(span);
		});

		paragraph.insertBefore(fragment, originalSegment);
		originalSegment.remove();

		reindexSegments();
		updateTranslationResultsAfterSplit(segmentIndex, newSegments);
		updateEditBar();
	}

	function updateSegmentsAfterJoin(
		leftSegment,
		rightSegment,
		newSegment,
		paragraphIndex,
		isPending = false,
	) {
		const leftIndex = parseInt(leftSegment.dataset.index, 10);
		const _rightIndex = parseInt(rightSegment.dataset.index, 10);

		// Remove both segments from pending set if they were there
		pendingSegments.delete(leftSegment);
		pendingSegments.delete(rightSegment);

		const span = document.createElement("span");
		span.className =
			"segment inline-block px-2 py-1 rounded border-2 border-transparent";
		span.style.fontFamily = "var(--font-chinese)";
		span.style.fontSize = "var(--text-chinese)";
		span.style.color = "var(--text-primary)";
		span.dataset.index = leftIndex;
		span.dataset.paragraph = paragraphIndex;
		span.dataset.pinyin = newSegment.pinyin;
		span.dataset.english = newSegment.english;
		span.textContent = newSegment.text;

		span.style.background = getPastelColor(leftIndex);
		span.style.cursor = "pointer";
		span.classList.add(
			"transition-all",
			"duration-150",
			"hover:-translate-y-px",
			"hover:shadow-sm",
		);

		if (isPending) {
			span.classList.add("segment-pending");
			pendingSegments.add(span);
		}

		addSegmentInteraction(span);

		// Remove join indicator between the two segments
		let el = leftSegment.nextElementSibling;
		while (el && el !== rightSegment) {
			const next = el.nextElementSibling;
			if (el.classList.contains("join-indicator")) {
				el.remove();
			}
			el = next;
		}

		leftSegment.parentNode.insertBefore(span, leftSegment);
		leftSegment.remove();
		rightSegment.remove();

		reindexSegments();
		updateTranslationResultsAfterJoin(leftIndex, newSegment);
		updateEditBar();
	}

	function reindexSegments() {
		let index = 0;

		document.querySelectorAll(".paragraph").forEach((para, paraIdx) => {
			para.querySelectorAll(".segment").forEach((seg) => {
				seg.dataset.index = index;
				seg.dataset.paragraph = paraIdx;
				if (
					!seg.classList.contains("saved") &&
					!seg.classList.contains("segment-pending") &&
					!seg.classList.contains("editing")
				) {
					seg.style.background = getPastelColor(index);
				}
				index++;
			});
		});
		// No need to update pendingSegments - it tracks DOM elements directly
	}

	function updateTranslationResultsAfterSplit(originalIndex, newSegments) {
		const translationResults = getTranslationResults();
		translationResults.splice(
			originalIndex,
			1,
			...newSegments.map((seg) => ({
				segment: seg.text,
				pinyin: seg.pinyin,
				english: seg.english,
			})),
		);
		setTranslationResults(translationResults);
		rebuildTranslationTable();
	}

	function updateTranslationResultsAfterJoin(leftIndex, newSegment) {
		const translationResults = getTranslationResults();
		translationResults.splice(leftIndex, 2, {
			segment: newSegment.text,
			pinyin: newSegment.pinyin,
			english: newSegment.english,
		});
		setTranslationResults(translationResults);
		rebuildTranslationTable();
	}

	function rebuildSegmentDisplay() {
		// This will be called by app.js to re-render segments
		if (window.App?.rebuildSegments) {
			window.App.rebuildSegments();
		}
	}

	function rebuildTranslationTable() {
		const tableContainer = document.getElementById("translation-table");
		const translationResults = getTranslationResults();
		if (!tableContainer || translationResults.length === 0) return;

		const wasExpanded = !document
			.getElementById("details-content")
			?.classList.contains("hidden");

		let tableHtml = `
            <div class="p-4 rounded-xl" style="background: var(--surface); box-shadow: 0 1px 3px var(--shadow); border: 1px solid var(--border);">
                <button id="toggle-details-btn" class="flex items-center justify-between w-full text-left">
                    <h3 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translation Details</h3>
                    <span id="toggle-icon" style="color: var(--text-muted); font-size: var(--text-lg);">${wasExpanded ? "−" : "+"}</span>
                </button>
                <div id="details-content" class="${wasExpanded ? "" : "hidden"} mt-3 overflow-x-auto">
                    <table class="w-full text-left">
                        <thead>
                            <tr style="border-bottom: 1px solid var(--border);">
                                <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">Chinese</th>
                                <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">Pinyin</th>
                                <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">English</th>
                            </tr>
                        </thead>
                        <tbody>
        `;

		translationResults.forEach((item, index) => {
			tableHtml += `
                <tr class="cursor-pointer translation-row" style="border-bottom: 1px solid var(--background-alt);" data-index="${index}">
                    <td class="py-2 px-2" style="font-family: var(--font-chinese); font-size: var(--text-chinese); color: var(--text-primary);">${item.segment}</td>
                    <td class="py-2 px-2" style="color: var(--text-secondary); font-size: var(--text-sm);">${item.pinyin}</td>
                    <td class="py-2 px-2" style="color: var(--secondary-dark); font-size: var(--text-sm);">${item.english}</td>
                </tr>
            `;
		});

		tableHtml += `
                        </tbody>
                    </table>
                </div>
            </div>
        `;
		tableContainer.innerHTML = tableHtml;

		document
			.getElementById("toggle-details-btn")
			?.addEventListener("click", () => {
				const content = document.getElementById("details-content");
				const icon = document.getElementById("toggle-icon");
				if (content && icon) {
					content.classList.toggle("hidden");
					icon.textContent = content.classList.contains("hidden") ? "+" : "−";
				}
			});
	}

	// ========================================
	// Public API
	// ========================================

	return {
		init: init,
		enterGlobalEditMode: enterGlobalEditMode,
		exitGlobalEditMode: exitGlobalEditMode,
		cancelEdits: cancelEdits,
		saveEdits: saveEdits,
		isEditModeActive: () => isEditModeActive,
	};
})();
