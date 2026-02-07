import { postJson } from "../../lib/api";
import type { TranslateBatchResponse } from "./types";

export async function translateBatch(
  segments: string[],
  context: string | null,
  jobId: string | null,
  paragraphIdx: number | null,
): Promise<TranslateBatchResponse> {
  return postJson("/api/segments/translate-batch", {
    segments,
    context,
    job_id: jobId,
    paragraph_idx: paragraphIdx,
  });
}
