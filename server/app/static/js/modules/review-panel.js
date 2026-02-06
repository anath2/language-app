/**
 * ReviewPanel Module - Flashcard review system
 */
const ReviewPanel = (() => {
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
		SRS.updateDueCount();
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

			State.set("reviewQueue", data.cards);
			State.set("reviewIndex", 0);

			if (data.cards.length === 0) {
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
					data.cards.length;
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
		const reviewQueue = State.get("reviewQueue");
		const reviewIndex = State.get("reviewIndex");
		const card = reviewQueue[reviewIndex];

		document.getElementById("review-current").textContent = reviewIndex + 1;
		State.set("reviewAnswered", false);

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
		if (State.get("reviewAnswered")) return;
		State.set("reviewAnswered", true);

		const reviewQueue = State.get("reviewQueue");
		const reviewIndex = State.get("reviewIndex");
		const card = reviewQueue[reviewIndex];

		try {
			await Api.postJson("/api/review/answer", {
				vocab_item_id: card.vocab_item_id,
				grade: grade,
			});
		} catch (e) {
			console.error("Failed to record grade:", e);
		}

		const newIndex = reviewIndex + 1;
		State.set("reviewIndex", newIndex);

		if (newIndex >= reviewQueue.length) {
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

	return {
		openReviewPanel,
		closeReviewPanel,
		loadReviewQueue,
		revealAnswer,
		gradeCard,
	};
})();
