/**
 * Segment Editor - Split/Join Functionality
 * Handles segment editing, splitting, joining, and undo operations
 */
const SegmentEditor = (function() {
    'use strict';

    // State
    let editingSegment = null;
    let lastUndoOperation = null;
    let activeEditPopover = null;

    // Dependencies (set by init)
    let getPastelColor = null;
    let addSegmentInteraction = null;
    let getTranslationResults = null;
    let setTranslationResults = null;

    // ========================================
    // Initialization
    // ========================================

    function init(deps) {
        getPastelColor = deps.getPastelColor;
        addSegmentInteraction = deps.addSegmentInteraction;
        getTranslationResults = deps.getTranslationResults;
        setTranslationResults = deps.setTranslationResults;

        // Delegate events for split/join functionality (click-based, no hover)
        document.addEventListener('click', handleEditClick, true);
        document.addEventListener('keydown', handleEditKeydown);
    }

    // ========================================
    // Edit Mode Functions
    // ========================================

    function enterSegmentEditMode(segment) {
        if (editingSegment) {
            exitSegmentEditMode();
        }

        // Don't allow editing single-character segments (can't split)
        const text = segment.textContent.trim();
        if (text.length < 1) return;

        editingSegment = segment;
        const originalBg = segment.style.background || getPastelColor(parseInt(segment.dataset.index) || 0);

        // Store original state
        segment.dataset.originalText = text;
        segment.dataset.originalBg = originalBg;

        // Build character wrappers with clickable split points between them
        let html = '';
        for (let i = 0; i < text.length; i++) {
            html += `<span class="char-wrapper" data-char-index="${i}" style="background: ${originalBg}; border-radius: 3px; padding: 0.25rem 0.15rem;">${text[i]}`;
            if (i < text.length - 1) {
                // Clickable split point between characters
                html += `<span class="split-point" data-split-after="${i}" title="Split here"></span>`;
            }
            html += `</span>`;
        }

        segment.innerHTML = html;
        segment.classList.add('editing');

        // Add join indicators on both sides
        addEditModeJoinIndicators(segment);

        // Add Done button below the segment
        addDoneButton(segment);
    }

    function exitSegmentEditMode() {
        if (!editingSegment) return;

        const segment = editingSegment;
        const originalText = segment.dataset.originalText;
        const originalBg = segment.dataset.originalBg;

        segment.innerHTML = originalText;
        segment.classList.remove('editing');
        segment.style.background = originalBg;

        // Remove join indicators and done button
        removeEditModeUI();

        editingSegment = null;
    }

    function addEditModeJoinIndicators(segment) {
        const paragraph = segment.parentNode;

        // Find previous valid segment
        let prevEl = segment.previousElementSibling;
        while (prevEl && (prevEl.classList.contains('join-indicator') || prevEl.classList.contains('edit-done-btn'))) {
            prevEl = prevEl.previousElementSibling;
        }

        // Add join indicator BEFORE segment (to join with previous)
        if (prevEl && prevEl.classList.contains('segment') && !prevEl.classList.contains('segment-pending')) {
            const joinLeft = document.createElement('span');
            joinLeft.className = 'join-indicator join-left visible';
            joinLeft.innerHTML = '⊕';
            joinLeft.dataset.direction = 'left';
            joinLeft.dataset.targetIndex = segment.dataset.index;
            joinLeft.title = 'Join with previous';
            paragraph.insertBefore(joinLeft, segment);
        }

        // Find next valid segment
        let nextEl = segment.nextElementSibling;
        while (nextEl && (nextEl.classList.contains('join-indicator') || nextEl.classList.contains('edit-done-btn'))) {
            nextEl = nextEl.nextElementSibling;
        }

        // Add join indicator AFTER segment (to join with next)
        if (nextEl && nextEl.classList.contains('segment') && !nextEl.classList.contains('segment-pending')) {
            const joinRight = document.createElement('span');
            joinRight.className = 'join-indicator join-right visible';
            joinRight.innerHTML = '⊕';
            joinRight.dataset.direction = 'right';
            joinRight.dataset.targetIndex = segment.dataset.index;
            joinRight.title = 'Join with next';
            paragraph.insertBefore(joinRight, segment.nextSibling);
        }
    }

    function addDoneButton(segment) {
        const doneBtn = document.createElement('button');
        doneBtn.className = 'edit-done-btn';
        doneBtn.innerHTML = '✓ Done';
        doneBtn.type = 'button';

        // Position it after the segment (and any join indicator)
        let insertAfter = segment.nextElementSibling;
        if (insertAfter && insertAfter.classList.contains('join-indicator')) {
            insertAfter = insertAfter.nextElementSibling;
        }

        if (insertAfter) {
            segment.parentNode.insertBefore(doneBtn, insertAfter);
        } else {
            segment.parentNode.appendChild(doneBtn);
        }
    }

    function removeEditModeUI() {
        // Remove all join indicators
        document.querySelectorAll('.join-indicator').forEach(el => el.remove());
        // Remove done button
        document.querySelectorAll('.edit-done-btn').forEach(el => el.remove());
    }

    // ========================================
    // Undo Support
    // ========================================

    function showUndoButton(targetSegmentIndex) {
        // Remove any existing undo button
        hideUndoButton();

        if (!lastUndoOperation) return;

        // Find the segment to position undo below
        const targetSegment = document.querySelector(`.segment[data-index="${targetSegmentIndex}"]`);
        if (!targetSegment) return;

        const undoBtn = document.createElement('button');
        undoBtn.className = 'undo-edit-btn';
        undoBtn.innerHTML = '↩';
        undoBtn.type = 'button';
        undoBtn.title = 'Undo';
        undoBtn.addEventListener('click', performUndo);

        // Add to the paragraph (which has position: relative)
        const paragraph = targetSegment.closest('.paragraph');
        if (paragraph) {
            paragraph.style.position = 'relative';
            paragraph.appendChild(undoBtn);

            // Position below the target segment
            const segRect = targetSegment.getBoundingClientRect();
            const paraRect = paragraph.getBoundingClientRect();

            undoBtn.style.left = (segRect.left - paraRect.left + segRect.width / 2 - 14) + 'px';
            undoBtn.style.top = (segRect.bottom - paraRect.top + 4) + 'px';
        }

        // Auto-hide after 8 seconds
        setTimeout(() => {
            if (lastUndoOperation) {
                hideUndoButton();
                lastUndoOperation = null;
            }
        }, 8000);
    }

    function hideUndoButton() {
        document.querySelectorAll('.undo-edit-btn').forEach(el => el.remove());
    }

    async function performUndo() {
        if (!lastUndoOperation) return;

        const undo = lastUndoOperation;
        lastUndoOperation = null;
        hideUndoButton();

        if (undo.type === 'split') {
            // Undo split = join the two segments back together
            const segments = document.querySelectorAll(`.segment[data-paragraph="${undo.paragraphIndex}"]`);
            let leftSeg = null, rightSeg = null;

            for (const seg of segments) {
                const idx = parseInt(seg.dataset.index);
                if (idx === undo.segmentIndex) {
                    leftSeg = seg;
                } else if (idx === undo.segmentIndex + 1) {
                    rightSeg = seg;
                }
            }

            if (leftSeg && rightSeg) {
                leftSeg.classList.add('loading');
                rightSeg.classList.add('loading');

                try {
                    const result = await stubJoinSegments(leftSeg.textContent, rightSeg.textContent);
                    updateSegmentsAfterJoin(leftSeg, rightSeg, {
                        text: undo.originalText,
                        pinyin: undo.pinyin || result.segment.pinyin,
                        english: undo.english || result.segment.english
                    }, undo.paragraphIndex);
                    console.log('Undo split completed');
                } catch (error) {
                    console.error('Undo split failed:', error);
                    leftSeg.classList.remove('loading');
                    rightSeg.classList.remove('loading');
                }
            }
        } else if (undo.type === 'join') {
            // Undo join = split the merged segment back
            const mergedSeg = document.querySelector(`.segment[data-index="${undo.leftIndex}"]`);

            if (mergedSeg) {
                mergedSeg.classList.add('loading');

                try {
                    updateSegmentsAfterSplit(mergedSeg, [
                        {
                            text: undo.leftText,
                            pinyin: undo.leftPinyin || generateStubPinyin(undo.leftText),
                            english: undo.leftEnglish || `[${undo.leftText}]`
                        },
                        {
                            text: undo.rightText,
                            pinyin: undo.rightPinyin || generateStubPinyin(undo.rightText),
                            english: undo.rightEnglish || `[${undo.rightText}]`
                        }
                    ], undo.paragraphIndex);
                    console.log('Undo join completed');
                } catch (error) {
                    console.error('Undo join failed:', error);
                    mergedSeg.classList.remove('loading');
                }
            }
        }
    }

    // ========================================
    // Event Handlers
    // ========================================

    function handleEditClick(e) {
        // Handle Done button click
        const doneBtn = e.target.closest('.edit-done-btn');
        if (doneBtn) {
            e.stopPropagation();
            exitSegmentEditMode();
            return;
        }

        // Handle split point click - DIRECT ACTION (no popover)
        const splitPoint = e.target.closest('.split-point');
        if (splitPoint) {
            e.stopPropagation();
            const segment = splitPoint.closest('.segment');
            const splitAfter = parseInt(splitPoint.dataset.splitAfter);
            performSplit(segment, splitAfter);
            return;
        }

        // Handle join indicator click - DIRECT ACTION (no popover)
        const joinIndicator = e.target.closest('.join-indicator');
        if (joinIndicator) {
            e.stopPropagation();
            const direction = joinIndicator.dataset.direction;
            const targetIndex = parseInt(joinIndicator.dataset.targetIndex);
            const targetSegment = document.querySelector(`.segment[data-index="${targetIndex}"]`);

            if (targetSegment) {
                performJoinFromEditMode(targetSegment, direction);
            }
            return;
        }

        // Handle popover button clicks (legacy)
        const popoverBtn = e.target.closest('.edit-popover-btn');
        if (popoverBtn) {
            e.stopPropagation();
            if (popoverBtn.classList.contains('cancel-btn')) {
                closeEditPopover();
            }
            return;
        }

        // Click outside editing segment - exit edit mode
        if (editingSegment && !e.target.closest('.segment.editing') &&
            !e.target.closest('.join-indicator') && !e.target.closest('.edit-done-btn')) {
            exitSegmentEditMode();
        }

        // Click outside popover - close it (legacy)
        if (activeEditPopover && !e.target.closest('.edit-popover')) {
            closeEditPopover();
        }
    }

    function handleEditKeydown(e) {
        // Escape exits edit mode
        if (e.key === 'Escape') {
            if (editingSegment) {
                exitSegmentEditMode();
            }
            if (activeEditPopover) {
                closeEditPopover();
            }
        }
    }

    // ========================================
    // Join Helper
    // ========================================

    function performJoinFromEditMode(segment, direction) {
        let leftSegment, rightSegment;

        if (direction === 'left') {
            rightSegment = segment;
            let prevEl = segment.previousElementSibling;
            while (prevEl && !prevEl.classList.contains('segment')) {
                prevEl = prevEl.previousElementSibling;
            }
            leftSegment = prevEl;
        } else {
            leftSegment = segment;
            let nextEl = segment.nextElementSibling;
            while (nextEl && !nextEl.classList.contains('segment')) {
                nextEl = nextEl.nextElementSibling;
            }
            rightSegment = nextEl;
        }

        if (leftSegment && rightSegment) {
            performJoin(
                parseInt(leftSegment.dataset.index),
                parseInt(rightSegment.dataset.index)
            );
        }
    }

    // ========================================
    // Legacy Popover Functions
    // ========================================

    function closeEditPopover() {
        if (activeEditPopover) {
            activeEditPopover.remove();
            activeEditPopover = null;
        }

        if (editingSegment) {
            exitSegmentEditMode();
        }
    }

    // ========================================
    // Stubbed API Calls
    // ========================================

    async function stubSplitSegment(segmentText, splitAfter) {
        await new Promise(resolve => setTimeout(resolve, 300));

        const leftText = segmentText.substring(0, splitAfter + 1);
        const rightText = segmentText.substring(splitAfter + 1);

        return {
            segments: [
                {
                    text: leftText,
                    pinyin: generateStubPinyin(leftText),
                    english: `[${leftText}]`
                },
                {
                    text: rightText,
                    pinyin: generateStubPinyin(rightText),
                    english: `[${rightText}]`
                }
            ],
            pending_translation: [0, 1]
        };
    }

    async function stubJoinSegments(leftText, rightText) {
        await new Promise(resolve => setTimeout(resolve, 300));

        const mergedText = leftText + rightText;

        return {
            segment: {
                text: mergedText,
                pinyin: generateStubPinyin(mergedText),
                english: `[${mergedText}]`
            },
            pending_translation: true
        };
    }

    function generateStubPinyin(text) {
        return `pinyin(${text})`;
    }

    // ========================================
    // Split/Join Operations
    // ========================================

    async function performSplit(segment, splitAfter) {
        const segmentIndex = parseInt(segment.dataset.index);
        const paragraphIndex = parseInt(segment.dataset.paragraph);
        const originalText = segment.dataset.originalText || segment.textContent.trim();

        const undoData = {
            type: 'split',
            originalText: originalText,
            segmentIndex: segmentIndex,
            paragraphIndex: paragraphIndex,
            pinyin: segment.dataset.pinyin,
            english: segment.dataset.english
        };

        exitSegmentEditMode();

        const targetSegment = document.querySelector(`.segment[data-index="${segmentIndex}"]`);
        if (!targetSegment) return;

        targetSegment.classList.add('loading');

        try {
            const result = await stubSplitSegment(originalText, splitAfter);
            closeEditPopover();
            updateSegmentsAfterSplit(targetSegment, result.segments, paragraphIndex);

            lastUndoOperation = undoData;
            showUndoButton(segmentIndex);

            console.log('Split completed:', result);
        } catch (error) {
            console.error('Split failed:', error);
            targetSegment.classList.remove('loading');
        }
    }

    async function performJoin(leftIndex, rightIndex) {
        let leftSegment = document.querySelector(`.segment[data-index="${leftIndex}"]`);
        let rightSegment = document.querySelector(`.segment[data-index="${rightIndex}"]`);

        if (!leftSegment || !rightSegment) return;

        const leftText = leftSegment.dataset.originalText || leftSegment.textContent.trim();
        const rightText = rightSegment.dataset.originalText || rightSegment.textContent.trim();
        const paragraphIndex = parseInt(leftSegment.dataset.paragraph);

        const undoData = {
            type: 'join',
            leftText: leftText,
            rightText: rightText,
            leftIndex: leftIndex,
            rightIndex: rightIndex,
            paragraphIndex: paragraphIndex,
            leftPinyin: leftSegment.dataset.pinyin,
            leftEnglish: leftSegment.dataset.english,
            rightPinyin: rightSegment.dataset.pinyin,
            rightEnglish: rightSegment.dataset.english
        };

        exitSegmentEditMode();

        leftSegment = document.querySelector(`.segment[data-index="${leftIndex}"]`);
        rightSegment = document.querySelector(`.segment[data-index="${rightIndex}"]`);

        if (!leftSegment || !rightSegment) return;

        leftSegment.classList.add('loading');
        rightSegment.classList.add('loading');

        try {
            const result = await stubJoinSegments(leftText, rightText);
            closeEditPopover();
            updateSegmentsAfterJoin(leftSegment, rightSegment, result.segment, paragraphIndex);

            lastUndoOperation = undoData;
            showUndoButton(leftIndex);

            console.log('Join completed:', result);
        } catch (error) {
            console.error('Join failed:', error);
            leftSegment.classList.remove('loading');
            rightSegment.classList.remove('loading');
        }
    }

    // ========================================
    // UI Update Functions
    // ========================================

    function updateSegmentsAfterSplit(originalSegment, newSegments, paragraphIndex) {
        const segmentIndex = parseInt(originalSegment.dataset.index);
        const paragraph = originalSegment.parentNode;

        const fragment = document.createDocumentFragment();

        newSegments.forEach((seg, i) => {
            const newIndex = segmentIndex + i;
            const span = document.createElement('span');
            span.className = 'segment inline-block px-2 py-1 rounded border-2 border-transparent';
            span.style.fontFamily = 'var(--font-chinese)';
            span.style.fontSize = 'var(--text-chinese)';
            span.style.color = 'var(--text-primary)';
            span.dataset.index = newIndex;
            span.dataset.paragraph = paragraphIndex;
            span.dataset.pinyin = seg.pinyin;
            span.dataset.english = seg.english;
            span.textContent = seg.text;

            span.style.background = getPastelColor(newIndex);
            span.style.cursor = 'pointer';
            span.classList.add('transition-all', 'duration-150', 'hover:-translate-y-px', 'hover:shadow-sm');

            addSegmentInteraction(span);

            fragment.appendChild(span);
        });

        paragraph.insertBefore(fragment, originalSegment);
        originalSegment.remove();

        reindexSegments();
        updateTranslationResultsAfterSplit(segmentIndex, newSegments);
    }

    function updateSegmentsAfterJoin(leftSegment, rightSegment, newSegment, paragraphIndex) {
        const leftIndex = parseInt(leftSegment.dataset.index);

        const span = document.createElement('span');
        span.className = 'segment inline-block px-2 py-1 rounded border-2 border-transparent';
        span.style.fontFamily = 'var(--font-chinese)';
        span.style.fontSize = 'var(--text-chinese)';
        span.style.color = 'var(--text-primary)';
        span.dataset.index = leftIndex;
        span.dataset.paragraph = paragraphIndex;
        span.dataset.pinyin = newSegment.pinyin;
        span.dataset.english = newSegment.english;
        span.textContent = newSegment.text;

        span.style.background = getPastelColor(leftIndex);
        span.style.cursor = 'pointer';
        span.classList.add('transition-all', 'duration-150', 'hover:-translate-y-px', 'hover:shadow-sm');

        addSegmentInteraction(span);

        leftSegment.parentNode.insertBefore(span, leftSegment);
        leftSegment.remove();
        rightSegment.remove();

        removeEditModeUI();
        reindexSegments();
        updateTranslationResultsAfterJoin(leftIndex, newSegment);
    }

    function reindexSegments() {
        let index = 0;
        document.querySelectorAll('.paragraph').forEach((para, paraIdx) => {
            para.querySelectorAll('.segment').forEach(seg => {
                seg.dataset.index = index;
                seg.dataset.paragraph = paraIdx;
                if (!seg.classList.contains('saved') && seg.dataset.pinyin) {
                    seg.style.background = getPastelColor(index);
                }
                index++;
            });
        });
    }

    function updateTranslationResultsAfterSplit(originalIndex, newSegments) {
        const translationResults = getTranslationResults();
        translationResults.splice(originalIndex, 1, ...newSegments.map(seg => ({
            segment: seg.text,
            pinyin: seg.pinyin,
            english: seg.english
        })));
        setTranslationResults(translationResults);
        rebuildTranslationTable();
    }

    function updateTranslationResultsAfterJoin(leftIndex, newSegment) {
        const translationResults = getTranslationResults();
        translationResults.splice(leftIndex, 2, {
            segment: newSegment.text,
            pinyin: newSegment.pinyin,
            english: newSegment.english
        });
        setTranslationResults(translationResults);
        rebuildTranslationTable();
    }

    function rebuildTranslationTable() {
        const tableContainer = document.getElementById('translation-table');
        const translationResults = getTranslationResults();
        if (!tableContainer || translationResults.length === 0) return;

        const wasExpanded = !document.getElementById('details-content')?.classList.contains('hidden');

        let tableHtml = `
            <div class="p-4 rounded-xl" style="background: var(--surface); box-shadow: 0 1px 3px var(--shadow); border: 1px solid var(--border);">
                <button id="toggle-details-btn" class="flex items-center justify-between w-full text-left">
                    <h3 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translation Details</h3>
                    <span id="toggle-icon" style="color: var(--text-muted); font-size: var(--text-lg);">${wasExpanded ? '−' : '+'}</span>
                </button>
                <div id="details-content" class="${wasExpanded ? '' : 'hidden'} mt-3 overflow-x-auto">
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

        document.getElementById('toggle-details-btn').addEventListener('click', () => {
            const content = document.getElementById('details-content');
            const icon = document.getElementById('toggle-icon');
            content.classList.toggle('hidden');
            icon.textContent = content.classList.contains('hidden') ? '+' : '−';
        });
    }

    // ========================================
    // Legacy Aliases
    // ========================================

    function enterSplitMode(segment) { enterSegmentEditMode(segment); }
    function exitSplitMode(segment) { if (editingSegment === segment) exitSegmentEditMode(); }
    function addJoinIndicators(segment) { addEditModeJoinIndicators(segment); }
    function removeJoinIndicators() { removeEditModeUI(); }

    // ========================================
    // Public API
    // ========================================

    return {
        init: init,
        enterEditMode: enterSegmentEditMode,
        exitEditMode: exitSegmentEditMode,
        // Legacy
        enterSplitMode: enterSplitMode,
        exitSplitMode: exitSplitMode,
        addJoinIndicators: addJoinIndicators,
        removeJoinIndicators: removeJoinIndicators
    };
})();
