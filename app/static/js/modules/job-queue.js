/**
 * JobQueue Module - Job queue management and streaming
 */
const JobQueue = (() => {
	async function loadJobQueue() {
		try {
			const response = await fetch("/api/jobs?limit=20");
			if (!response.ok) throw new Error("Failed to load jobs");
			const data = await response.json();
			State.set("jobQueue", data.jobs);
			renderJobQueue();
		} catch (e) {
			console.error("Failed to load job queue:", e);
		}
	}

	function renderJobQueue() {
		const jobList = document.getElementById("job-list");
		const queueCount = document.getElementById("queue-count");
		const jobQueue = State.get("jobQueue");
		const currentJobId = State.get("currentJobId");

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

		const timeAgo = Utils.formatTimeAgo(job.created_at);
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
                <div class="job-preview">${Utils.escapeHtml(job.input_preview)}</div>
                ${job.full_translation_preview ? `<div class="job-translation-preview">"${Utils.escapeHtml(job.full_translation_preview)}"</div>` : ""}
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
			TranslationUI.showError(`Failed to submit translation job: ${e.message}`);
			return null;
		}
	}

	async function expandJob(jobId) {
		State.set("currentJobId", jobId);
		State.set("isExpandedView", true);

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
			State.set("currentRawText", job.input_text);
			State.set("currentTextId", null);

			// If job is completed, render results
			if (job.status === "completed" && job.paragraphs) {
				TranslationUI.renderCompletedJob(job);
			} else if (job.status === "processing" || job.status === "pending") {
				// Stream progress
				streamJobProgress(jobId);
			} else if (job.status === "failed") {
				TranslationUI.showError(job.error_message || "Job failed");
			}

			// Update mini queue preview
			renderJobQueue();
		} catch (e) {
			console.error("Failed to expand job:", e);
			TranslationUI.showError("Failed to load job details");
		}
	}

	async function streamJobProgress(jobId) {
		TranslationUI.showLoadingState();

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
								TranslationUI.renderProgressUI(
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
								TranslationUI.updateProgress(data.current, data.total);
								TranslationUI.updateSegment(data.result);
								// Update job card in queue
								updateJobCardProgress(jobId, data.current, data.total);
								break;

							case "complete":
								TranslationUI.finalizeUI(data.paragraphs);
								if (data.fullTranslation) {
									const translationEl =
										document.getElementById("full-translation");
									if (translationEl) {
										translationEl.textContent = data.fullTranslation;
									}
								}
								await SRS.fetchAndApplySRSInfo();
								SRS.applyPostStreamStyling(SegmentInteractions.addSegmentInteraction);
								// Reload job queue to update status
								await loadJobQueue();
								break;

							case "error":
								TranslationUI.showError(data.message);
								await loadJobQueue();
								break;
						}
					}
				}
			}
		} catch (error) {
			TranslationUI.showError(`Streaming failed: ${error.message}`);
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
		State.set("isExpandedView", false);
		State.set("currentJobId", null);

		// Show queue panel, hide expanded view
		const queuePanel = document.getElementById("job-queue-panel");
		const expandedView = document.getElementById("expanded-job-view");
		if (queuePanel) queuePanel.classList.remove("hidden");
		if (expandedView) expandedView.classList.add("hidden");

		// Clear results
		TranslationUI.showEmptyState();

		// Reload queue
		loadJobQueue();
	}

	async function deleteJob(jobId) {
		if (!confirm("Delete this translation?")) return;

		try {
			const response = await fetch(`/api/jobs/${jobId}`, { method: "DELETE" });
			if (!response.ok) throw new Error("Failed to delete job");

			// If we're viewing this job, go back to queue
			if (State.get("currentJobId") === jobId) {
				backToQueue();
			} else {
				await loadJobQueue();
			}
		} catch (e) {
			console.error("Failed to delete job:", e);
			alert("Failed to delete job");
		}
	}

	return {
		loadJobQueue,
		renderJobQueue,
		submitJob,
		expandJob,
		backToQueue,
		deleteJob,
	};
})();
