/**
 * TranslationUI Module - Translation rendering and progress UI
 */
const TranslationUI = (() => {
	function renderProgressUI(paragraphs, _totalSegments, fullTranslation = "") {
		const resultsDiv = document.getElementById("results");
		let html = `
            <div class="relative space-y-3">
                <div class="space-y-1">
                    <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Translation</h2>
                    <p id="full-translation" class="full-translation">Translating...</p>
                </div>

                <div class="section-divider my-3">
                    <span>Segmented Text</span>
                </div>
                <div class="flex items-center justify-between mb-3">
                    <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Segmented Text</h2>
                    <button id="edit-segments-btn" class="btn-edit" type="button">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                        </svg>
                        Edit
                    </button>
                </div>

                <div class="progress-container">
                    <div class="flex justify-between mb-1.5" style="font-size: var(--text-xs);">
                        <span id="progress-label" style="color: var(--text-secondary);">Segmenting...</span>
                        <span id="progress-count" style="color: var(--text-secondary);"></span>
                    </div>
                    <div class="progress-bar-bg">
                        <div id="progress-bar" class="progress-bar-fill" style="width: 0%"></div>
                    </div>
                </div>
               <div id="segments-container">
        `;

		let globalIndex = 0;
		paragraphs.forEach((para, paraIdx) => {
			const marginBottom = para.separator
				? para.separator.split("\n").length * 0.4
				: 0;
			const paddingLeft = para.indent ? para.indent.length * 0.5 : 0;
			html += `<div class="paragraph flex flex-wrap gap-1" style="margin-bottom: ${marginBottom}rem; padding-left: ${paddingLeft}rem;">`;

			for (let i = 0; i < para.segment_count; i++) {
				html += `
                    <span class="segment segment-pending inline-block px-2 py-1 rounded border-2 border-transparent"
                          style="font-family: var(--font-chinese); font-size: var(--text-chinese);"
                          data-index="${globalIndex}"
                          data-paragraph="${paraIdx}">Loading...</span>
                `;
				globalIndex++;
			}

			html += `</div>`;
		});

		html += `
                </div>
                <!-- Floating tooltip overlay -->
                <div id="word-tooltip" class="word-tooltip hidden">
                    <div class="tooltip-pinyin" id="tooltip-pinyin"></div>
                    <div class="tooltip-english" id="tooltip-english"></div>
                    <div class="tooltip-actions">
                        <button id="save-word-btn" type="button" class="tooltip-btn">
                            Save to Learn
                        </button>
                        <button id="mark-known-btn" type="button" class="tooltip-btn hidden">
                            Mark as Known
                        </button>
                        <button id="resume-learning-btn" type="button" class="tooltip-btn hidden">
                            Resume Learning
                        </button>
                        <span id="save-word-status" class="tooltip-status"></span>
                    </div>
                    <div class="tooltip-arrow"></div>
                </div>
            </div>
        `;

		resultsDiv.innerHTML = html;

		const translationEl = document.getElementById("full-translation");
		if (translationEl) {
			translationEl.textContent = fullTranslation || "Translating...";
		}
	}

	function updateProgress(current, total) {
		const progressBar = document.getElementById("progress-bar");
		const progressLabel = document.getElementById("progress-label");
		const progressCount = document.getElementById("progress-count");

		const percentage = (current / total) * 100;
		progressBar.style.width = `${percentage}%`;
		progressLabel.textContent = "Translating...";
		progressCount.textContent = `${current} / ${total}`;
	}

	function updateSegment(result) {
		const segment = document.querySelector(
			`.segment[data-index="${result.index}"]`,
		);
		if (segment) {
			segment.textContent = result.segment;
			segment.classList.remove("segment-pending", "segment-translating");
			segment.style.color = "var(--text-primary)";
			segment.style.background = "transparent";
			segment.style.cursor = "default";
			segment.dataset.pinyin = result.pinyin;
			segment.dataset.english = result.english;
		}

		const nextSegment = document.querySelector(
			`.segment[data-index="${result.index + 1}"]`,
		);
		if (nextSegment?.classList.contains("segment-pending")) {
			nextSegment.classList.add("segment-translating");
		}
	}

	function finalizeUI(paragraphs) {
		const results = paragraphs.flatMap((p) => p.translations);
		State.set("translationResults", results);

		const progressContainer = document.querySelector(".progress-container");
		if (progressContainer) {
			progressContainer.style.display = "none";
		}

		const tableContainer = document.getElementById("translation-table");
		if (tableContainer) {
			let tableHtml = `
                <div class="p-4 rounded-xl" style="background: var(--surface); box-shadow: 0 1px 3px var(--shadow); border: 1px solid var(--border);">
                    <button id="toggle-details-btn" class="flex items-center justify-between w-full text-left">
                        <h3 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translation Details</h3>
                        <span id="toggle-icon" style="color: var(--text-muted); font-size: var(--text-lg);">+</span>
                    </button>
                    <div id="details-content" class="hidden mt-3 overflow-x-auto">
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

			results.forEach((item, index) => {
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
				.addEventListener("click", () => {
					const content = document.getElementById("details-content");
					const icon = document.getElementById("toggle-icon");
					content.classList.toggle("hidden");
					icon.textContent = content.classList.contains("hidden") ? "+" : "−";
				});
		}
	}

	function renderCompletedJob(job) {
		// Build translation results from job paragraphs
		const translationResults = [];
		if (job.paragraphs) {
			job.paragraphs.forEach((para) => {
				para.translations.forEach((t) => {
					translationResults.push({
						segment: t.segment,
						pinyin: t.pinyin,
						english: t.english,
					});
				});
			});
		}
		State.set("translationResults", translationResults);

		// Render the UI similar to streaming complete
		const resultsDiv = document.getElementById("results");
		let html = `
            <div class="relative space-y-3">
                <div class="space-y-1">
                    <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Translation</h2>
                    <p id="full-translation" class="full-translation">${Utils.escapeHtml(job.full_translation || "")}</p>
                </div>

                <div class="section-divider my-3">
                    <span>Segmented Text</span>
                </div>
                <div class="flex items-center justify-between mb-3">
                    <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Segmented Text</h2>
                    <button id="edit-segments-btn" class="btn-edit" type="button">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                        </svg>
                        Edit
                    </button>
                </div>
                <div id="segments-container">
        `;

		let globalIndex = 0;
		if (job.paragraphs) {
			job.paragraphs.forEach((para, paraIdx) => {
				const marginBottom = para.separator
					? para.separator.split("\n").length * 0.4
					: 0;
				const paddingLeft = para.indent ? para.indent.length * 0.5 : 0;
				html += `<div class="paragraph flex flex-wrap gap-1" style="margin-bottom: ${marginBottom}rem; padding-left: ${paddingLeft}rem;">`;

				para.translations.forEach((t) => {
					html += `
                        <span class="segment inline-block px-2 py-1 rounded border-2 border-transparent transition-all duration-150 hover:-translate-y-px hover:shadow-sm"
                              style="font-family: var(--font-chinese); font-size: var(--text-chinese); color: var(--text-primary); cursor: pointer;"
                              data-index="${globalIndex}"
                              data-paragraph="${paraIdx}"
                              data-pinyin="${Utils.escapeHtml(t.pinyin)}"
                              data-english="${Utils.escapeHtml(t.english)}">${Utils.escapeHtml(t.segment)}</span>
                    `;
					globalIndex++;
				});

				html += `</div>`;
			});
		}

		html += `
                </div>
                <!-- Floating tooltip overlay -->
                <div id="word-tooltip" class="word-tooltip hidden">
                    <div class="tooltip-pinyin" id="tooltip-pinyin"></div>
                    <div class="tooltip-english" id="tooltip-english"></div>
                    <div class="tooltip-actions">
                        <button id="save-word-btn" type="button" class="tooltip-btn">Save to Learn</button>
                        <button id="mark-known-btn" type="button" class="tooltip-btn hidden">Mark as Known</button>
                        <button id="resume-learning-btn" type="button" class="tooltip-btn hidden">Resume Learning</button>
                        <span id="save-word-status" class="tooltip-status"></span>
                    </div>
                    <div class="tooltip-arrow"></div>
                </div>
            </div>
        `;

		resultsDiv.innerHTML = html;

		// Add interactions and styling
		document.querySelectorAll(".segment").forEach((seg, idx) => {
			seg.style.background = Utils.getPastelColor(idx);
			SegmentInteractions.addSegmentInteraction(seg);
		});

		SRS.fetchAndApplySRSInfo();
		SRS.applyPostStreamStyling(SegmentInteractions.addSegmentInteraction);
		SegmentInteractions.setupEditButton();
	}

	function showError(message) {
		const resultsDiv = document.getElementById("results");
		resultsDiv.innerHTML = `
            <div class="p-3 rounded-md" style="background: var(--error); border-left: 3px solid var(--secondary-dark);">
                <p style="color: var(--text-primary); font-size: var(--text-sm);">${message}</p>
            </div>
        `;
	}

	function showLoadingState() {
		const resultsDiv = document.getElementById("results");
		resultsDiv.innerHTML = `
            <div class="h-full flex items-center justify-center">
                <div class="text-center">
                    <div class="spinner mx-auto mb-2" style="width: 20px; height: 20px; border-color: rgba(124, 158, 178, 0.3); border-top-color: var(--primary);"></div>
                    <p style="color: var(--text-muted); font-size: var(--text-sm);">Starting translation...</p>
                </div>
            </div>
        `;
	}

	function showEmptyState() {
		const resultsDiv = document.getElementById("results");
		resultsDiv.innerHTML = `
            <div class="h-full flex items-center justify-center">
                <p class="text-center italic" style="color: var(--text-muted); font-size: var(--text-sm);">Translation results will appear here</p>
            </div>
        `;
	}

	return {
		renderProgressUI,
		updateProgress,
		updateSegment,
		finalizeUI,
		renderCompletedJob,
		showError,
		showLoadingState,
		showEmptyState,
	};
})();
