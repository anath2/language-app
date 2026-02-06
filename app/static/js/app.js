/**
 * Language App - Main Application Orchestrator
 * Coordinates modules and sets up event handlers
 */
(() => {
	// ========================================
	// Image Upload Handlers
	// ========================================

	function clearPreview() {
		const fileInput = document.getElementById("image-input");
		const uploadPrompt = document.getElementById("upload-prompt");
		const previewContainer = document.getElementById("preview-container");

		fileInput.value = "";
		uploadPrompt.classList.remove("hidden");
		previewContainer.classList.add("hidden");
	}

	function setupImageUpload() {
		const dropZone = document.getElementById("drop-zone");
		const fileInput = document.getElementById("image-input");
		const uploadPrompt = document.getElementById("upload-prompt");
		const previewContainer = document.getElementById("preview-container");
		const imagePreview = document.getElementById("image-preview");
		const fileName = document.getElementById("file-name");

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
	}

	// ========================================
	// Segment Rebuild (for edit mode cancel)
	// ========================================

	function rebuildSegments() {
		// Clear stale tooltip state to prevent issues with old DOM references
		State.set("isClickActive", false);
		State.set("activeSegment", null);
		const tooltip = document.getElementById("word-tooltip");
		if (tooltip) tooltip.classList.add("hidden");

		const translationResults = State.get("translationResults");

		// Re-render segments from translationResults
		const segments = document.querySelectorAll(".segment");
		if (segments.length !== translationResults.length) {
			// Structure changed, need full rebuild within paragraphs
			document.querySelectorAll(".paragraph").forEach((para) => {
				para.innerHTML = "";
			});

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
				span.style.background = Utils.getPastelColor(idx);
				span.style.cursor = "pointer";
				span.classList.add(
					"transition-all",
					"duration-150",
					"hover:-translate-y-px",
					"hover:shadow-sm",
				);
				SegmentInteractions.addSegmentInteraction(span);

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
					seg.style.background = Utils.getPastelColor(idx);
				}
			});
		}

		// Re-apply SRS styling
		SRS.applyPostStreamStyling(SegmentInteractions.addSegmentInteraction);
	}

	// ========================================
	// Event Handlers Setup
	// ========================================

	function setupEventHandlers() {
		// Load initial due count
		SRS.updateDueCount();

		// Initialize segment interactions (sets up global click handlers)
		SegmentInteractions.init();

		// Load job queue on page load
		JobQueue.loadJobQueue();

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
			await JobQueue.submitJob(text);

			// Clear the input after submission
			textInput.value = "";

			translateBtn.querySelector(".btn-text").classList.remove("hidden");
			translateBtn.querySelector(".btn-loading").classList.add("hidden");
			translateBtn.disabled = false;
		});

		// Setup image upload handlers
		setupImageUpload();

		// Initialize segment editing functionality
		SegmentEditor.init({
			getPastelColor: Utils.getPastelColor,
			addSegmentInteraction: SegmentInteractions.addSegmentInteraction,
			getTranslationResults: () => State.get("translationResults"),
			setTranslationResults: (results) => {
				State.set("translationResults", results);
			},
			getJobId: () => State.get("currentJobId"),
		});
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
		openReviewPanel: ReviewPanel.openReviewPanel,
		closeReviewPanel: ReviewPanel.closeReviewPanel,
		loadReviewQueue: ReviewPanel.loadReviewQueue,
		revealAnswer: ReviewPanel.revealAnswer,
		gradeCard: ReviewPanel.gradeCard,
		// Job queue
		loadJobQueue: JobQueue.loadJobQueue,
		expandJob: JobQueue.expandJob,
		backToQueue: JobQueue.backToQueue,
		deleteJob: JobQueue.deleteJob,
		// Utilities
		clearPreview: clearPreview,
		rebuildSegments: rebuildSegments,
		get currentRawText() {
			return State.get("currentRawText");
		},
		get currentJobId() {
			return State.get("currentJobId");
		},
	};
})();
