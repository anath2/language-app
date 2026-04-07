# UI Issues Log

> Page under review: http://localhost:5173/translations/1771836357827273000
> Date: 2026-02-24

---

## Issues

### Issue 1: "Translated Text" starts with raw "translation:" prefix
- **Location**: Left panel, Translated Text section
- **Description**: The translated text begins with `translation:` as raw text on its own line before the actual translation. This looks like an unstripped label/key from the LLM response.
- **Screenshot**: Initial page load

### Issue 2: Segmented text spacing — single characters treated as isolated tokens
- **Location**: Right panel, Segmented Text
- **Description**: Single characters like `这`, `座`, `的`, `之`, `一`, `就`, `是`, `从`, `一`, `的` etc. are rendered as individual unhighlighted tokens with large gaps between them, making the text look very spaced out and difficult to read as a natural sentence.
- **Expected**: Non-vocabulary filler characters could be displayed inline without extra padding/spacing.

### Issue 6: Title edit mode has no visible editing state
- **Location**: Page title "About Edinburg" in the header bar
- **Description**: Clicking the title activates an inline text input, but the visual appearance does not change — no border, underline, background change, or any indicator that the field is now editable. The user has no feedback that they are in edit mode.
- **Expected**: The input should show a visible focus state (border, outline, or underline) when active.

### Issue 5: Segment popup cannot be dismissed with Escape key
- **Location**: Word detail popup after clicking a segment
- **Description**: Pressing Escape does not close the popup. Expected: Escape should dismiss modal/popup elements.

### Issue 4: Segment tooltip overlaps page title / header area
- **Location**: Right panel, clicking any segment pill
- **Description**: The word detail popup (pinyin + translation + Save/Mark buttons) appears at the top of the viewport, overlapping the page title ("About Edinburg") and potentially obscuring navigation. There is no close (×) button on the popup. The popup should either appear near the clicked element or have a clear dismiss affordance.
- **Screenshot**: After clicking 爱丁堡 segment

### Issue 7: Wrong CEDICT definition shown in segment popup — surname instead of common meaning
- **Location**: Segment popup for "于" (preposition "in/at") and "能" (modal "can/able")
- **Description**: Clicking "于" shows `Yú / surname Yu` and "能" shows `Néng / surname Neng`. Both are using the rare proper-noun (surname) reading from CEDICT instead of the far more common meanings (于 = "in/at", 能 = "can"). This will confuse learners.
- **Expected**: The most contextually relevant / most common definition should be shown, not the first CEDICT entry.

### Issue 8: Pinyin format inconsistency in Translation Details table
- **Location**: Translation Details table, Pinyin column
- **Description**: Some entries show pinyin wrapped in square brackets (e.g. `[dōu]`, `[zhè]`) while most show it without brackets (e.g. `jiù`, `shì`). This is visually inconsistent.

### Issue 9: "Comments:" label treated as a vocabulary segment in Translation Details table
- **Location**: Translation Details table
- **Description**: "Comments:" appears as a row in the translation table with no pinyin or English, alongside Chinese vocabulary. It's a metadata label from the source text, not a vocabulary item.

### Issue 10: Punctuation and non-word tokens clutter Translation Details table
- **Location**: Translation Details table
- **Description**: Punctuation marks (，、""), date strings (2026/2), ellipsis (...), and emoji (🥹) all appear as rows in the table with empty Pinyin and English cells. This creates significant noise and makes the table hard to scan for actual vocabulary. These non-vocabulary tokens should be filtered out.

### Issue 11: Translation Details table English definitions are excessively verbose
- **Location**: Translation Details table, English column
- **Description**: Common function words like "的", "就", "是" show full multi-line CEDICT dictionary entries spanning many alternatives. These walls of text make the table hard to read. A shorter gloss or the first definition only should be shown.

### Issue 3: "2026/2" is not segmented/highlighted but surrounded by highlighted segments
- **Location**: Right panel, line with "屑杂鱼，2026/2，于爱丁堡"
- **Description**: The date "2026/2" appears as plain text between highlighted segments. The surrounding text segments (屑杂鱼, 于, 爱丁堡) have colored pill styling, but the date is unstyled and difficult to differentiate visually.

### Issue 12: CEDICT redirect definitions leak into UI
- **Location**: Translation Details table, rows for 从
- **Description**: 从 shows `variant of 從|从[cong2]` — a raw CEDICT cross-reference entry, not an actual definition. This is useless for learners and looks like a data formatting error.
- **Expected**: Follow the redirect and show the target entry's definitions.

### Issue 13: Translation Details table shows duplicate rows for repeated words
- **Location**: Translation Details table
- **Description**: Words that appear multiple times in the text are listed once per occurrence (e.g. 就×2, 从×2, 城市×2, 看到×2, 的×4, 很×3, 美好×2, 地方×2, 爱丁堡×3). This makes the table far longer than necessary and adds no learning value.
- **Expected**: Deduplicate: show each unique word once.

### Issue 14: 屑杂鱼 pinyin has capital letter mid-word in Translation Details
- **Location**: Translation Details table, 屑杂鱼 row
- **Description**: Pinyin is displayed as `xiè zá Yú` with a capital Y. This appears to be a character-by-character CEDICT lookup where 鱼 matched a surname entry (Yú). Mid-word capitalization is incorrect for a common noun pinyin rendering.
- **Expected**: All syllables in a non-proper-noun word should be lowercase.

### Issue 15: 都 pinyin/definition mismatch — bracket format returns wrong entry
- **Location**: Translation Details table, row for 都
- **Description**: Pinyin shows `[dōu]` (in brackets, suggesting a fallback path) but the definition says "surname Du" — which would be pronounced Dū not dōu. The two fields contradict each other. The expected reading is dōu meaning "all/both".

### Issue 16: 地方 shows political/administrative definition instead of common "place" meaning
- **Location**: Translation Details table and segment popup for 地方
- **Description**: 地方 is shown with definition "region / regional (away from the central administration) / local". In this text's context (美好的地方 = "beautiful place"), the correct reading is dì·fang meaning "place / location". The first CEDICT entry returned is the wrong sense of the word.

### Issue 18: Pencil/edit buttons are inconsistent across the two panels

- **Location**: Left panel "Edit source text" button vs right panel "Segmented Text" edit button
- **Description**: Both buttons use the same Lucide pencil icon but are implemented with completely different button styles:

  | Property | Left (Edit source text) | Right (Segmented Text) |
  |----------|------------------------|------------------------|
  | CSS class | `edit-icon-btn` (custom) | `btn btn-ghost btn-xs btn-pill` (design system) |
  | Size | 24×20px | 42×26px |
  | Shape | Slightly rounded square (5.6px radius) | Fully pill-shaped (9999px radius) |
  | Icon color | Dark `rgb(29,35,46)` | Muted slate `rgb(100,116,139)` |
  | Hover state | None (no visual feedback) | Background fills to light grey |
  | `aria-label` | ✅ "Edit source text" | ❌ Missing |

- **Expected**: Both buttons should use the same design system component (`btn btn-ghost btn-xs`) with consistent size, color, hover behavior, and accessible labels. The left button also has a very small hit area (24×20px) and no hover feedback, making it hard to discover.

### Issue 17: Segment popup has no close button
- **Location**: Word detail popup (e.g. after clicking 爱丁堡)
- **Description**: The popup shows pinyin + definition + "Save to Learn" / "Mark as Known" buttons, but there is no × close button. The only way to dismiss it appears to be clicking elsewhere or pressing Escape (which also doesn't work per Issue 5). Users have no clear affordance to dismiss the popup.

### Issue 19: Nav bar mixes custom class and design system — account button misaligned
- **Location**: Top navigation bar
- **Description**: The three text nav buttons ("Translate", "Projects", "Explore") use a custom `nav-item` class with no `btn` base. The account/avatar icon button uses `btn btn-ghost nav-item account-btn` — mixing design system and custom class. This results in a height mismatch: account button renders at **40px** vs **34px** for text nav items, causing vertical misalignment. The account button is also icon-only with **no `aria-label`**, making it inaccessible.
- **Expected**: All nav items should use the same base class. Account button needs an `aria-label` (e.g. "Account").

### Issue 20: "Translation Details" toggle missing ARIA expanded state
- **Location**: "Translation Details" collapsible section button
- **Description**: Uses a custom `table-toggle` class (not design system) with no `aria-expanded` attribute. Screen readers cannot detect whether the section is open or collapsed.
- **Expected**: Button should set `aria-expanded="true"/"false"` toggling with state, using a consistent design system disclosure pattern.

### Issue 21: Right panel has a structural header wrapper that the left panel lacks
- **Location**: Left panel ("Original Text" / "Translated Text") vs right panel ("Segmented Text")
- **Description**: The right panel wraps its title and edit button in a `.panel-header` div, giving it a horizontal bar layout. The left panel has no `.panel-header` — its labels use `.panel-label` directly. This structural asymmetry means the two panels are built differently despite appearing visually similar, making the left panel harder to extend with header-level actions consistently.
- **Expected**: Both panels should use the same structural template (panel-header + panel-label + optional action buttons).

