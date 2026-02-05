/**
 * Language App - Core Application Logic
 * Handles translation, SRS, review panel, and UI interactions
 */
(() => {
	// ========================================
	// Persistence State
	// ========================================

	let currentTextId = null;
	let currentRawText = "";

	// ========================================
	// Pastel Colors for Segments
	// ========================================

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

	// ========================================
	// Translation State
	// ========================================

	let translationResults = [];
	let isClickActive = false;
	let activeSegment = null;
	let lastTooltipData = null;

	// ========================================
	// SRS State
	// ========================================

	const savedVocabMap = new Map();
	let reviewQueue = [];
	let reviewIndex = 0;
	let reviewAnswered = false;

	// ========================================
	// Job Queue State
	// ========================================

	let jobQueue = [];
	let currentJobId = null;
	let _isExpandedView = false;

	// ========================================
	// API Helpers
	// ========================================

	async function apiPostJson(path, payload) {
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
		if (currentTextId || !currentRawText) return currentTextId;
		const data = await apiPostJson("/api/texts", {
			raw_text: currentRawText,
			source_type: "text",
			metadata: {},
		});
		currentTextId = data.id;
		return currentTextId;
	}

	async function logEvent(eventType, payload = {}) {
		try {
			await apiPostJson("/api/events", {
				event_type: eventType,
				text_id: currentTextId,
				payload,
			});
		} catch (_e) {
			// swallow
		}
	}

	function clearPreview() {
		const fileInput = document.getElementById("image-input");
		const uploadPrompt = document.getElementById("upload-prompt");
		const previewContainer = document.getElementById("preview-container");

		fileInput.value = "";
		uploadPrompt.classList.remove("hidden");
		previewContainer.classList.add("hidden");
	}

	// ========================================
	// Job Queue Functions
	// ========================================

	async function loadJobQueue() {
		try {
			const response = await fetch("/api/jobs?limit=20");
			if (!response.ok) throw new Error("Failed to load jobs");
			const data = await response.json();
			jobQueue = data.jobs;
			renderJobQueue();
		} catch (e) {
			console.error("Failed to load job queue:", e);
		}
	}

	function renderJobQueue() {
		const jobList = document.getElementById("job-list");
		const queueCount = document.getElementById("queue-count");

		if (!jobList) return;

		queueCount.textContent = `${jobQueue.length} job${jobQueue.length !== 1 ? "s" : ""}`;

		if (jobQueue.length === 0) {
			jobList.innerHTML = `
                <div class="text-center py-8">
                    <p class="italic" style="color: var(--text-muted); font-size: var(--text-sm);">No translation jobs yet</p>
                    <p class="mt-1" style="color: var(--text-muted); font-size: var(--text-xs);">Submit text on the left to start</p>
                </div>
            `;
			return;
		}

		jobList.innerHTML = jobQueue.map((job) => renderJobCard(job)).join("");

		// Update mini queue preview if in expanded view
		const miniQueueCount = document.getElementById("mini-queue-count");
		if (miniQueueCount) {
			const otherJobs = jobQueue.filter((j) => j.id !== currentJobId).length;
			miniQueueCount.textContent = `${otherJobs} more`;
			const miniPreview = document.getElementById("mini-queue-preview");
			if (miniPreview) {
				miniPreview.classList.toggle("hidden", otherJobs === 0);
			}
		}
	}

	function renderJobCard(job) {
		const statusLabels = {
			pending: "Pending",
			processing: "Processing",
			completed: "Completed",
			failed: "Failed",
		};

		const timeAgo = formatTimeAgo(job.created_at);
		const progressHtml =
			job.status === "processing" && job.total_segments
				? `<div class="job-progress"><div class="job-progress-fill" style="width: ${(job.segment_count / job.total_segments) * 100}%"></div></div>`
				: "";

		const segmentInfo =
			job.segment_count !== null && job.total_segments !== null
				? `${job.segment_count} / ${job.total_segments} segments`
				: job.status === "completed" && job.segment_count
					? `${job.segment_count} segments`
					: "";

		return `
            <div class="job-card ${job.status}" data-job-id="${job.id}" onclick="window.App.expandJob('${job.id}')">
                <div class="job-header">
                    <div class="job-status">
                        <span class="job-status-icon"></span>
                        <span style="color: var(--text-secondary);">${statusLabels[job.status]}</span>
                    </div>
                    <span class="job-time">${timeAgo}</span>
                </div>
                <div class="job-preview">${escapeHtml(job.input_preview)}</div>
                ${job.full_translation_preview ? `<div class="job-translation-preview">"${escapeHtml(job.full_translation_preview)}"</div>` : ""}
                ${progressHtml}
                <div class="job-footer">
                    <span class="job-segments-count">${segmentInfo}</span>
                    <button class="job-delete-btn" onclick="event.stopPropagation(); window.App.deleteJob('${job.id}');" title="Delete">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
                        </svg>
                    </button>
                </div>
            </div>
        `;
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

	function escapeHtml(text) {
		const div = document.createElement("div");
		div.textContent = text;
		return div.innerHTML;
	}

	async function submitJob(text) {
		try {
			const response = await fetch("/api/jobs", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ input_text: text, source_type: "text" }),
			});
			if (!response.ok) throw new Error("Failed to create job");
			const data = await response.json();

			// Add to queue and render
			await loadJobQueue();

			// Expand the new job
			expandJob(data.job_id);

			return data.job_id;
		} catch (e) {
			console.error("Failed to submit job:", e);
			showError(`Failed to submit translation job: ${e.message}`);
			return null;
		}
	}

	async function expandJob(jobId) {
		currentJobId = jobId;
		_isExpandedView = true;

		// Hide queue panel, show expanded view
		const queuePanel = document.getElementById("job-queue-panel");
		const expandedView = document.getElementById("expanded-job-view");
		if (queuePanel) queuePanel.classList.add("hidden");
		if (expandedView) expandedView.classList.remove("hidden");

		// Fetch job details
		try {
			const response = await fetch(`/api/jobs/${jobId}`);
			if (!response.ok) throw new Error("Failed to load job");
			const job = await response.json();

			// Set original text
			const originalTextContent = document.getElementById(
				"original-text-content",
			);
			if (originalTextContent) {
				originalTextContent.textContent = job.input_text;
			}

			// Store for potential editing
			currentRawText = job.input_text;
			currentTextId = null;

			// If job is completed, render results
			if (job.status === "completed" && job.paragraphs) {
				renderCompletedJob(job);
			} else if (job.status === "processing" || job.status === "pending") {
				// Stream progress
				streamJobProgress(jobId);
			} else if (job.status === "failed") {
				showError(job.error_message || "Job failed");
			}

			// Update mini queue preview
			renderJobQueue();
		} catch (e) {
			console.error("Failed to expand job:", e);
			showError("Failed to load job details");
		}
	}

	function renderCompletedJob(job) {
		// Build translation results from job paragraphs
		translationResults = [];
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

		// Render the UI similar to streaming complete
		const resultsDiv = document.getElementById("results");
		let html = `
            <div class="relative space-y-3">
                <div class="space-y-1">
                    <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Translation</h2>
                    <p id="full-translation" class="full-translation">${escapeHtml(job.full_translation || "")}</p>
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
                              data-pinyin="${escapeHtml(t.pinyin)}"
                              data-english="${escapeHtml(t.english)}">${escapeHtml(t.segment)}</span>
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
			seg.style.background = getPastelColor(idx);
			addSegmentInteraction(seg);
		});

		fetchAndApplySRSInfo();
		applyPostStreamStyling();
		setupEditButton();
	}

	async function streamJobProgress(jobId) {
		const resultsDiv = document.getElementById("results");
		resultsDiv.innerHTML = `
            <div class="h-full flex items-center justify-center">
                <div class="text-center">
                    <div class="spinner mx-auto mb-2" style="width: 20px; height: 20px; border-color: rgba(124, 158, 178, 0.3); border-top-color: var(--primary);"></div>
                    <p style="color: var(--text-muted); font-size: var(--text-sm);">Starting translation...</p>
                </div>
            </div>
        `;

		try {
			const response = await fetch(`/jobs/${jobId}/stream`);
			const reader = response.body.getReader();
			const decoder = new TextDecoder();
			let buffer = "";

			while (true) {
				const { value, done } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const lines = buffer.split("\n");
				buffer = lines.pop() || "";

				for (const line of lines) {
					if (line.startsWith("data: ")) {
						const data = JSON.parse(line.slice(6));

						switch (data.type) {
							case "start": {
								renderProgressUI(
									data.paragraphs,
									data.total,
									data.fullTranslation,
								);
								const firstSeg = document.querySelector(
									'.segment[data-index="0"]',
								);
								if (firstSeg) firstSeg.classList.add("segment-translating");
								break;
							}

							case "progress":
								updateProgress(data.current, data.total);
								updateSegment(data.result);
								// Update job card in queue
								updateJobCardProgress(jobId, data.current, data.total);
								break;

							case "complete":
								finalizeUI(data.paragraphs);
								if (data.fullTranslation) {
									const translationEl =
										document.getElementById("full-translation");
									if (translationEl) {
										translationEl.textContent = data.fullTranslation;
									}
								}
								await fetchAndApplySRSInfo();
								applyPostStreamStyling();
								// Reload job queue to update status
								await loadJobQueue();
								break;

							case "error":
								showError(data.message);
								await loadJobQueue();
								break;
						}
					}
				}
			}
		} catch (error) {
			showError(`Streaming failed: ${error.message}`);
			await loadJobQueue();
		}
	}

	function updateJobCardProgress(jobId, current, total) {
		const card = document.querySelector(`.job-card[data-job-id="${jobId}"]`);
		if (!card) return;

		const progressBar = card.querySelector(".job-progress-fill");
		if (progressBar) {
			progressBar.style.width = `${(current / total) * 100}%`;
		}

		const segCount = card.querySelector(".job-segments-count");
		if (segCount) {
			segCount.textContent = `${current} / ${total} segments`;
		}
	}

	function backToQueue() {
		_isExpandedView = false;
		currentJobId = null;

		// Show queue panel, hide expanded view
		const queuePanel = document.getElementById("job-queue-panel");
		const expandedView = document.getElementById("expanded-job-view");
		if (queuePanel) queuePanel.classList.remove("hidden");
		if (expandedView) expandedView.classList.add("hidden");

		// Clear results
		const resultsDiv = document.getElementById("results");
		if (resultsDiv) {
			resultsDiv.innerHTML = `
                <div class="h-full flex items-center justify-center">
                    <p class="text-center italic" style="color: var(--text-muted); font-size: var(--text-sm);">Translation results will appear here</p>
                </div>
            `;
		}

		// Reload queue
		loadJobQueue();
	}

	async function deleteJob(jobId) {
		if (!confirm("Delete this translation?")) return;

		try {
			const response = await fetch(`/api/jobs/${jobId}`, { method: "DELETE" });
			if (!response.ok) throw new Error("Failed to delete job");

			// If we're viewing this job, go back to queue
			if (currentJobId === jobId) {
				backToQueue();
			} else {
				await loadJobQueue();
			}
		} catch (e) {
			console.error("Failed to delete job:", e);
			alert("Failed to delete job");
		}
	}

	// ========================================
	// SRS Functions
	// ========================================

	async function recordLookup(vocabItemId, segmentEl) {
		try {
			const data = await apiPostJson("/api/vocab/lookup", {
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
				if (savedVocabMap.has(headword)) {
					savedVocabMap.get(headword).opacity = data.opacity;
					savedVocabMap.get(headword).isStruggling = data.is_struggling;
				}
			}
		} catch (e) {
			console.error("Failed to record lookup:", e);
		}
	}

	function updateAllSegmentInstances(
		headword,
		vocabItemId,
		opacity,
		isStruggling,
	) {
		document.querySelectorAll(".segment").forEach((seg) => {
			if (seg.textContent === headword) {
				seg.classList.add("saved");
				seg.dataset.vocabItemId = vocabItemId;

				const originalColor =
					seg.style.getPropertyValue("--segment-color") ||
					seg.style.background ||
					getPastelColor(parseInt(seg.dataset.index, 10) || 0);
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
		const segments = document.querySelectorAll(
			".segment:not(.segment-pending)",
		);
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
						getPastelColor(parseInt(seg.dataset.index, 10) || 0);

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

	function applyPostStreamStyling() {
		const segments = document.querySelectorAll(
			".segment:not(.segment-pending)",
		);
		segments.forEach((seg) => {
			if (!seg.dataset.pinyin && !seg.dataset.english) {
				seg.style.background = "transparent";
				seg.style.cursor = "default";
				return;
			}

			const headword = seg.textContent;
			const info = savedVocabMap.get(headword);

			if (info) {
				const originalColor = getPastelColor(
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
				seg.style.background = getPastelColor(
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
			addSegmentInteraction(seg);
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

	// ========================================
	// Review Panel Functions
	// ========================================

	async function openReviewPanel() {
		const panel = document.getElementById("review-panel");
		const overlay = document.getElementById("panel-overlay");
		panel.classList.add("open");
		overlay.classList.add("visible");
		await loadReviewQueue();
	}

	function closeReviewPanel() {
		const panel = document.getElementById("review-panel");
		const overlay = document.getElementById("panel-overlay");
		panel.classList.remove("open");
		overlay.classList.remove("visible");
		updateDueCount();
	}

	async function loadReviewQueue() {
		const content = document.getElementById("review-panel-content");
		const progress = document.getElementById("review-progress");

		content.innerHTML = `
            <div class="text-center py-8">
                <div class="spinner mx-auto" style="width: 24px; height: 24px; border-color: rgba(124, 158, 178, 0.3); border-top-color: var(--primary);"></div>
                <p class="mt-2" style="color: var(--text-muted); font-size: var(--text-sm);">Loading cards...</p>
            </div>
        `;

		try {
			const res = await fetch("/api/review/queue?limit=20");
			if (!res.ok) throw new Error("Failed to load queue");
			const data = await res.json();

			reviewQueue = data.cards;
			reviewIndex = 0;

			if (reviewQueue.length === 0) {
				content.innerHTML = `
                    <div class="review-empty">
                        <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                        </svg>
                        <p class="font-medium" style="font-size: var(--text-base);">All caught up!</p>
                        <p style="font-size: var(--text-sm);">No cards due for review right now.</p>
                    </div>
                `;
				progress.classList.add("hidden");
			} else {
				document.getElementById("review-total").textContent =
					reviewQueue.length;
				progress.classList.remove("hidden");
				renderReviewCard();
			}
		} catch (_e) {
			content.innerHTML = `
                <div class="review-empty">
                    <p style="color: var(--error);">Failed to load review cards.</p>
                </div>
            `;
		}
	}

	function renderReviewCard() {
		const content = document.getElementById("review-panel-content");
		const card = reviewQueue[reviewIndex];

		document.getElementById("review-current").textContent = reviewIndex + 1;
		reviewAnswered = false;

		const snippetHtml =
			card.snippets && card.snippets.length > 0
				? `<div class="snippet">"${card.snippets[0]}"</div>`
				: "";

		content.innerHTML = `
            <div class="review-card">
                <div class="headword">${card.headword}</div>
                <button class="reveal-btn" onclick="window.App.revealAnswer()">Show Answer</button>
                <div id="answer-section" class="answer-section hidden">
                    <div class="pinyin">${card.pinyin}</div>
                    <div class="english">${card.english}</div>
                    ${snippetHtml}
                    <div class="grade-buttons">
                        <button class="grade-btn again" onclick="window.App.gradeCard(0)">Again</button>
                        <button class="grade-btn hard" onclick="window.App.gradeCard(1)">Hard</button>
                        <button class="grade-btn good" onclick="window.App.gradeCard(2)">Good</button>
                    </div>
                </div>
            </div>
        `;
	}

	function revealAnswer() {
		document.getElementById("answer-section").classList.remove("hidden");
		document.querySelector(".reveal-btn").classList.add("hidden");
	}

	async function gradeCard(grade) {
		if (reviewAnswered) return;
		reviewAnswered = true;

		const card = reviewQueue[reviewIndex];

		try {
			await apiPostJson("/api/review/answer", {
				vocab_item_id: card.vocab_item_id,
				grade: grade,
			});
		} catch (e) {
			console.error("Failed to record grade:", e);
		}

		reviewIndex++;
		if (reviewIndex >= reviewQueue.length) {
			const content = document.getElementById("review-panel-content");
			content.innerHTML = `
                <div class="review-empty">
                    <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                    </svg>
                    <p class="font-medium" style="font-size: var(--text-base);">Session Complete!</p>
                    <p style="font-size: var(--text-sm);">You've reviewed ${reviewQueue.length} cards.</p>
                    <button class="btn-primary mt-4 px-4 py-2" onclick="window.App.loadReviewQueue()">Continue Reviewing</button>
                </div>
            `;
			document.getElementById("review-progress").classList.add("hidden");
		} else {
			renderReviewCard();
		}
	}

	// ========================================
	// Translation UI Functions
	// ========================================

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

	function addSegmentInteraction(segment) {
		// Prevent duplicate event listeners
		if (segment.dataset.hasInteraction === "true") return;
		segment.dataset.hasInteraction = "true";

		const tooltip = document.getElementById("word-tooltip");
		const tooltipPinyin = document.getElementById("tooltip-pinyin");
		const tooltipEnglish = document.getElementById("tooltip-english");

		function showTooltip(seg) {
			document.querySelectorAll(".segment").forEach((s) => {
				s.classList.remove("border-gray-800", "shadow-lg");
			});

			tooltipPinyin.textContent = seg.dataset.pinyin;
			tooltipEnglish.textContent = seg.dataset.english;
			lastTooltipData = {
				headword: seg.textContent,
				pinyin: seg.dataset.pinyin || "",
				english: seg.dataset.english || "",
			};
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
			tooltip.classList.add("hidden");
			document.querySelectorAll(".segment").forEach((s) => {
				s.classList.remove("border-gray-800", "shadow-lg");
			});
		}

		segment.addEventListener("mouseenter", () => {
			if (!isClickActive) {
				showTooltip(segment);
			}
		});

		segment.addEventListener("mouseleave", () => {
			if (!isClickActive) {
				hideTooltip();
			}
		});

		segment.addEventListener("click", (e) => {
			e.stopPropagation();
			if (isClickActive && activeSegment === segment) {
				isClickActive = false;
				activeSegment = null;
				hideTooltip();
			} else {
				isClickActive = true;
				activeSegment = segment;
				showTooltip(segment);
				logEvent("tap", { headword: segment.textContent });

				if (segment.dataset.vocabItemId) {
					recordLookup(segment.dataset.vocabItemId, segment);
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

	function finalizeUI(paragraphs) {
		const results = paragraphs.flatMap((p) => p.translations);
		translationResults = results;

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

	function showError(message) {
		const resultsDiv = document.getElementById("results");
		resultsDiv.innerHTML = `
            <div class="p-3 rounded-md" style="background: var(--error); border-left: 3px solid var(--secondary-dark);">
                <p style="color: var(--text-primary); font-size: var(--text-sm);">${message}</p>
            </div>
        `;
	}

	// ========================================
	// Main Translation Function
	// ========================================

	async function _translateWithProgress(text) {
		const resultsDiv = document.getElementById("results");
		const tableContainer = document.getElementById("translation-table");

		tableContainer.innerHTML = "";
		isClickActive = false;
		activeSegment = null;
		lastTooltipData = null;
		currentTextId = null;
		currentRawText = text;
		resultsDiv.innerHTML = `
            <div class="h-full flex items-center justify-center">
                <div class="text-center">
                    <div class="spinner mx-auto mb-2" style="width: 20px; height: 20px; border-color: rgba(124, 158, 178, 0.3); border-top-color: var(--primary);"></div>
                    <p style="color: var(--text-muted); font-size: var(--text-sm);">Segmenting text...</p>
                </div>
            </div>
        `;

		try {
			const formData = new FormData();
			formData.append("text", text);

			const response = await fetch("/translate-stream", {
				method: "POST",
				body: formData,
			});

			const reader = response.body.getReader();
			const decoder = new TextDecoder();
			let buffer = "";

			while (true) {
				const { value, done } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const lines = buffer.split("\n");
				buffer = lines.pop() || "";

				for (const line of lines) {
					if (line.startsWith("data: ")) {
						const data = JSON.parse(line.slice(6));

						switch (data.type) {
							case "start": {
								renderProgressUI(
									data.paragraphs,
									data.total,
									data.fullTranslation,
								);
								const firstSeg = document.querySelector(
									'.segment[data-index="0"]',
								);
								if (firstSeg) firstSeg.classList.add("segment-translating");
								break;
							}

							case "progress":
								updateProgress(data.current, data.total);
								updateSegment(data.result);
								break;

							case "complete":
								finalizeUI(data.paragraphs);
								if (data.fullTranslation) {
									const translationEl =
										document.getElementById("full-translation");
									if (translationEl) {
										translationEl.textContent = data.fullTranslation;
									}
								}
								try {
									await ensureSavedText();
								} catch (_e) {}
								await fetchAndApplySRSInfo();
								applyPostStreamStyling();
								break;

							case "error":
								showError(data.message);
								break;
						}
					}
				}
			}
		} catch (error) {
			showError(`Translation failed: ${error.message}`);
		}
	}

	// ========================================
	// Event Handlers Setup
	// ========================================

	function setupEventHandlers() {
		// Load initial due count
		updateDueCount();

		// Save button inside tooltip
		document.addEventListener("click", async (e) => {
			if (e.target && e.target.id === "save-word-btn") {
				e.stopPropagation();
				const statusEl = document.getElementById("save-word-status");
				try {
					await ensureSavedText();
					if (!lastTooltipData || !lastTooltipData.headword) return;
					const result = await apiPostJson("/api/vocab/save", {
						headword: lastTooltipData.headword,
						pinyin: lastTooltipData.pinyin,
						english: lastTooltipData.english,
						text_id: currentTextId,
						snippet: currentRawText,
						status: "learning",
					});
					if (statusEl) statusEl.textContent = "Saved";
					logEvent("save_vocab", { headword: lastTooltipData.headword });

					if (result.vocab_item_id) {
						savedVocabMap.set(lastTooltipData.headword, {
							vocabItemId: result.vocab_item_id,
							opacity: 1.0,
							isStruggling: false,
							status: "learning",
						});

						updateAllSegmentInstances(
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

					updateDueCount();
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
					if (!activeSegment || !activeSegment.dataset.vocabItemId) return;
					const vocabItemId = activeSegment.dataset.vocabItemId;
					await apiPostJson("/api/vocab/status", {
						vocab_item_id: vocabItemId,
						status: "known",
					});
					if (statusEl) statusEl.textContent = "Marked known";
					const headword = activeSegment.textContent;
					logEvent("mark_known", { headword: headword });

					if (savedVocabMap.has(headword)) {
						const info = savedVocabMap.get(headword);
						info.status = "known";
						info.opacity = 0;
					}

					updateAllSegmentInstances(headword, vocabItemId, 0, false);

					document.getElementById("mark-known-btn").classList.add("hidden");
					document
						.getElementById("resume-learning-btn")
						.classList.remove("hidden");

					updateDueCount();
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
					if (!activeSegment || !activeSegment.dataset.vocabItemId) return;
					const vocabItemId = activeSegment.dataset.vocabItemId;
					await apiPostJson("/api/vocab/status", {
						vocab_item_id: vocabItemId,
						status: "learning",
					});
					if (statusEl) statusEl.textContent = "Resumed";
					const headword = activeSegment.textContent;
					logEvent("resume_learning", { headword: headword });

					if (savedVocabMap.has(headword)) {
						const info = savedVocabMap.get(headword);
						info.status = "learning";
						info.opacity = 1.0;
					}

					updateAllSegmentInstances(headword, vocabItemId, 1.0, false);

					document
						.getElementById("resume-learning-btn")
						.classList.add("hidden");
					document.getElementById("mark-known-btn").classList.remove("hidden");

					updateDueCount();
				} catch (_err) {
					if (statusEl) statusEl.textContent = "Error";
				}
			}
		});

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

		// Load job queue on page load
		loadJobQueue();

		// Translate form handler - uses job queue
		const translateForm = document.getElementById("translate-form");
		const translateBtn = translateForm.querySelector('button[type="submit"]');

		translateForm.addEventListener("submit", async (e) => {
			e.preventDefault();
			const textInput = document.getElementById("text");
			const text = textInput.value.trim();
			if (!text) return;

			translateBtn.querySelector(".btn-text").classList.add("hidden");
			translateBtn.querySelector(".btn-loading").classList.remove("hidden");
			translateBtn.disabled = true;

			// Submit to job queue instead of direct translation
			await submitJob(text);

			// Clear the input after submission
			textInput.value = "";

			translateBtn.querySelector(".btn-text").classList.remove("hidden");
			translateBtn.querySelector(".btn-loading").classList.add("hidden");
			translateBtn.disabled = false;
		});

		// Image upload handlers
		const dropZone = document.getElementById("drop-zone");
		const fileInput = document.getElementById("image-input");
		const uploadPrompt = document.getElementById("upload-prompt");
		const previewContainer = document.getElementById("preview-container");
		const imagePreview = document.getElementById("image-preview");
		const fileName = document.getElementById("file-name");

		dropZone.addEventListener("click", (e) => {
			if (e.target.tagName !== "BUTTON") {
				fileInput.click();
			}
		});

		dropZone.addEventListener("dragover", (e) => {
			e.preventDefault();
			dropZone.classList.add("drag-over");
		});

		dropZone.addEventListener("dragleave", () => {
			dropZone.classList.remove("drag-over");
		});

		dropZone.addEventListener("drop", (e) => {
			e.preventDefault();
			dropZone.classList.remove("drag-over");

			const files = e.dataTransfer.files;
			if (files.length > 0) {
				fileInput.files = files;
				showPreview(files[0]);
			}
		});

		fileInput.addEventListener("change", (e) => {
			if (e.target.files.length > 0) {
				showPreview(e.target.files[0]);
			}
		});

		function showPreview(file) {
			const reader = new FileReader();
			reader.onload = (e) => {
				imagePreview.src = e.target.result;
				fileName.textContent = file.name;
				uploadPrompt.classList.add("hidden");
				previewContainer.classList.remove("hidden");
			};
			reader.readAsDataURL(file);
		}

		// Click outside segments to dismiss pinned tooltip
		document.addEventListener("click", (e) => {
			const tooltip = document.getElementById("word-tooltip");
			const clickedInTooltip = tooltip?.contains(e.target);
			if (isClickActive && !clickedInTooltip && !e.target.closest(".segment")) {
				isClickActive = false;
				activeSegment = null;
				if (tooltip) {
					tooltip.classList.add("hidden");
				}
				document.querySelectorAll(".segment").forEach((s) => {
					s.classList.remove("border-gray-800", "shadow-lg");
				});
			}
		});

		// Initialize segment editing functionality
		SegmentEditor.init({
			getPastelColor: getPastelColor,
			addSegmentInteraction: addSegmentInteraction,
			getTranslationResults: () => translationResults,
			setTranslationResults: (results) => {
				translationResults = results;
			},
			getJobId: () => currentJobId,
		});
	}

	// ========================================
	// Segment Rebuild Function (for edit mode cancel)
	// ========================================

	function rebuildSegments() {
		// Clear stale tooltip state to prevent issues with old DOM references
		isClickActive = false;
		activeSegment = null;
		const tooltip = document.getElementById("word-tooltip");
		if (tooltip) tooltip.classList.add("hidden");

		// Re-render segments from translationResults
		const segments = document.querySelectorAll(".segment");
		if (segments.length !== translationResults.length) {
			// Structure changed, need full rebuild within paragraphs
			document.querySelectorAll(".paragraph").forEach((para) => {
				para.innerHTML = "";
			});

			const _index = 0;
			const paragraphs = document.querySelectorAll(".paragraph");

			translationResults.forEach((result, idx) => {
				// Find appropriate paragraph (use first one if structure is unclear)
				const para =
					paragraphs[0] || document.querySelector("#segments-container");
				if (!para) return;

				const span = document.createElement("span");
				span.className =
					"segment inline-block px-2 py-1 rounded border-2 border-transparent";
				span.style.fontFamily = "var(--font-chinese)";
				span.style.fontSize = "var(--text-chinese)";
				span.style.color = "var(--text-primary)";
				span.dataset.index = idx;
				span.dataset.paragraph = "0";
				span.dataset.pinyin = result.pinyin;
				span.dataset.english = result.english;
				span.textContent = result.segment;
				span.style.background = getPastelColor(idx);
				span.style.cursor = "pointer";
				span.classList.add(
					"transition-all",
					"duration-150",
					"hover:-translate-y-px",
					"hover:shadow-sm",
				);
				addSegmentInteraction(span);

				para.appendChild(span);
			});
		} else {
			// Same count, just update content and styling
			segments.forEach((seg, idx) => {
				const result = translationResults[idx];
				if (result) {
					seg.textContent = result.segment;
					seg.dataset.pinyin = result.pinyin;
					seg.dataset.english = result.english;
					seg.classList.remove("segment-pending", "editing");
					seg.style.background = getPastelColor(idx);
				}
			});
		}

		// Re-apply SRS styling
		applyPostStreamStyling();
	}

	// ========================================
	// Initialization
	// ========================================

	document.addEventListener("DOMContentLoaded", setupEventHandlers);

	// ========================================
	// Public API (for inline onclick handlers)
	// ========================================

	window.App = {
		// Review panel
		openReviewPanel: openReviewPanel,
		closeReviewPanel: closeReviewPanel,
		loadReviewQueue: loadReviewQueue,
		revealAnswer: revealAnswer,
		gradeCard: gradeCard,
		// Job queue
		loadJobQueue: loadJobQueue,
		expandJob: expandJob,
		backToQueue: backToQueue,
		deleteJob: deleteJob,
		// Utilities
		clearPreview: clearPreview,
		rebuildSegments: rebuildSegments,
		get currentRawText() {
			return currentRawText;
		},
		get currentJobId() {
			return currentJobId;
		},
	};
})();
