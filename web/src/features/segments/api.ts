import { postJson } from "../../lib/api";
import type { TranslateBatchResponse } from "./types";

export async function translateBatch(
  segments: string[],
  context: string | null,
  translationId: string | null,
  paragraphIdx: number | null,
): Promise<TranslateBatchResponse> {
  return postJson("/api/segments/translate-batch", {
    segments,
    context,
    translation_id: translationId,
    paragraph_idx: paragraphIdx,
  });
}
