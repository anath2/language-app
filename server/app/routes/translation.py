"""
Translation and OCR routes.

Endpoints:
- POST /translate-text - JSON translation endpoint
- POST /translate-html - HTMX translation endpoint
- POST /translate-stream - SSE streaming translation endpoint
- POST /extract-text-html - HTMX OCR extraction
- POST /translate-image - JSON image translation
- POST /translate-image-html - HTMX image translation
"""

import json

from fastapi import APIRouter, File, Form, HTTPException, Request, UploadFile
from fastapi.responses import HTMLResponse, StreamingResponse

from app.models import (
    ParagraphResult,
    ExtractTextResponse,
    TranslateRequest,
    TranslateResponse,
    TranslationResult,
)
from app.cedict import lookup
from app.pipeline import get_full_translator, get_pipeline
from app.templates_config import templates
from app.utils import (
    extract_text_from_image,
    should_skip_segment,
    split_into_paragraphs,
    to_pinyin,
    validate_image_file,
)

router = APIRouter(tags=["translation"])


@router.post("/translate-text", response_model=TranslateResponse)
async def translate_text(request: TranslateRequest):
    """Translate Chinese text to Pinyin and English"""
    pipe = get_pipeline()

    # Split text into paragraphs
    paragraphs = split_into_paragraphs(request.text)

    # Process each paragraph through the pipeline
    paragraph_results = []
    for para in paragraphs:
        results = await pipe.aforward(para["content"])
        translations = [
            TranslationResult(segment=seg, pinyin=pinyin, english=english)
            for seg, pinyin, english in results
        ]
        paragraph_results.append(
            ParagraphResult(
                translations=translations,
                indent=para.get("indent", ""),
                separator=para["separator"],
            )
        )

    return TranslateResponse(paragraphs=paragraph_results)


@router.post("/translate-html", response_class=HTMLResponse)
async def translate_html(request: Request, text: str = Form(...)):
    """Translate Chinese text and return HTML fragment for HTMX."""
    if not text.strip():
        return templates.TemplateResponse(
            request=request,
            name="fragments/error.html",
            context={"message": "Please enter some Chinese text"},
        )

    try:
        pipe = get_pipeline()

        # Split text into paragraphs
        paragraphs = split_into_paragraphs(text)

        # Process each paragraph through the pipeline
        paragraph_results = []
        for para in paragraphs:
            results = await pipe.aforward(para["content"])
            translations = [
                {"segment": seg, "pinyin": pinyin, "english": english}
                for seg, pinyin, english in results
            ]
            paragraph_results.append(
                {
                    "translations": translations,
                    "indent": para.get("indent", ""),
                    "separator": para["separator"],
                }
            )

        return templates.TemplateResponse(
            request=request,
            name="fragments/results.html",
            context={"paragraphs": paragraph_results, "original_text": text},
        )
    except Exception as e:
        return templates.TemplateResponse(
            request=request,
            name="fragments/error.html",
            context={"message": f"Translation error: {e}"},
        )


@router.post("/translate-stream")
async def translate_stream(text: str = Form(...)):
    """Stream translation progress via SSE"""

    async def generate():
        if not text.strip():
            yield f"data: {json.dumps({'type': 'error', 'message': 'Please enter some Chinese text'})}\n\n"
            return

        try:
            pipe = get_pipeline()
            full_translator = get_full_translator()

            # Full-text translation (separate from pipeline)
            full_translation_result = await full_translator.acall(text=text)
            full_translation = full_translation_result.english

            # Split text into paragraphs
            paragraphs = split_into_paragraphs(text)

            # Count total segments across all paragraphs
            all_paragraph_segments = []
            for para in paragraphs:
                segmentation = await pipe.segment.acall(text=para["content"])
                all_paragraph_segments.append(
                    {
                        "segments": segmentation.segments,
                        "indent": para.get("indent", ""),
                        "separator": para["separator"],
                    }
                )

            total_segments = sum(len(p["segments"]) for p in all_paragraph_segments)

            # Send initial info with paragraph structure
            paragraph_info = [
                {
                    "segment_count": len(p["segments"]),
                    "indent": p["indent"],
                    "separator": p["separator"],
                }
                for p in all_paragraph_segments
            ]
            yield f"data: {json.dumps({'type': 'start', 'total': total_segments, 'paragraphs': paragraph_info, 'fullTranslation': full_translation})}\n\n"

            # Step 2: Translate each segment in each paragraph
            global_index = 0
            all_results = []

            for para_idx, para_data in enumerate(all_paragraph_segments):
                para_results = []
                for seg_idx, segment in enumerate(para_data["segments"]):
                    # Skip translation for segments with only symbols, numbers, and punctuation
                    if should_skip_segment(segment):
                        result = {
                            "segment": segment,
                            "pinyin": "",
                            "english": "",
                            "index": global_index,
                            "paragraph_index": para_idx,
                        }
                    else:
                        # Pinyin is generated deterministically; LLM only provides English
                        pinyin = to_pinyin(segment)
                        # Use the original paragraph content as context
                        sentence_context = paragraphs[para_idx]["content"]
                        # Look up dictionary definition
                        dict_entry = lookup(pipe.cedict, segment) or "Not in dictionary"
                        translation = await pipe.translate.acall(
                            segment=segment,
                            sentence_context=sentence_context,
                            dictionary_entry=dict_entry,
                        )
                        result = {
                            "segment": segment,
                            "pinyin": pinyin,
                            "english": translation.english,
                            "index": global_index,
                            "paragraph_index": para_idx,
                        }
                    para_results.append(result)
                    global_index += 1

                    # Send progress update
                    yield f"data: {json.dumps({'type': 'progress', 'current': global_index, 'total': total_segments, 'result': result})}\n\n"

                all_results.append(
                    {
                        "translations": para_results,
                        "indent": para_data["indent"],
                        "separator": para_data["separator"],
                    }
                )

            # Send completion with paragraph structure
            yield f"data: {json.dumps({'type': 'complete', 'paragraphs': all_results, 'fullTranslation': full_translation})}\n\n"

        except Exception as e:
            yield f"data: {json.dumps({'type': 'error', 'message': str(e)})}\n\n"

    return StreamingResponse(generate(), media_type="text/event-stream")


@router.post("/extract-text-html", response_class=HTMLResponse)
async def extract_text_html(request: Request, file: UploadFile = File(...)):
    """HTMX endpoint for OCR extraction only - fills textarea for editing"""
    try:
        file_bytes = await file.read()

        valid, error = validate_image_file(file_bytes, file.filename or "image.png")
        if not valid:
            return templates.TemplateResponse(
                request=request,
                name="fragments/error.html",
                context={"message": error},
            )

        extracted_text = await extract_text_from_image(file_bytes)

        if not extracted_text.strip():
            return templates.TemplateResponse(
                request=request,
                name="fragments/error.html",
                context={"message": "No Chinese text found in image"},
            )

        return templates.TemplateResponse(
            request=request,
            name="fragments/ocr-result.html",
            context={"extracted_text": extracted_text},
        )
    except Exception as e:
        return templates.TemplateResponse(
            request=request,
            name="fragments/error.html",
            context={"message": f"OCR error: {e}"},
        )


@router.post("/extract-text", response_model=ExtractTextResponse)
async def extract_text_json(file: UploadFile = File(...)):
    """JSON endpoint for OCR extraction only."""
    file_bytes = await file.read()

    valid, error = validate_image_file(file_bytes, file.filename or "image.png")
    if not valid:
        raise HTTPException(status_code=400, detail=error)

    extracted_text = await extract_text_from_image(file_bytes)

    if not extracted_text.strip():
        raise HTTPException(status_code=400, detail="No Chinese text found in image")

    return ExtractTextResponse(text=extracted_text)


@router.post("/translate-image", response_model=TranslateResponse)
async def translate_image(file: UploadFile = File(...)):
    """Extract Chinese text from image and translate"""
    file_bytes = await file.read()

    valid, error = validate_image_file(file_bytes, file.filename or "image.png")
    if not valid:
        raise HTTPException(status_code=400, detail=error)

    extracted_text = await extract_text_from_image(file_bytes)

    if not extracted_text.strip():
        raise HTTPException(status_code=400, detail="No Chinese text found in image")

    pipe = get_pipeline()

    # Split text into paragraphs
    paragraphs = split_into_paragraphs(extracted_text)

    # Process each paragraph through the pipeline
    paragraph_results = []
    for para in paragraphs:
        results = await pipe.aforward(para["content"])
        translations = [
            TranslationResult(segment=seg, pinyin=pinyin, english=english)
            for seg, pinyin, english in results
        ]
        paragraph_results.append(
            ParagraphResult(
                translations=translations,
                indent=para.get("indent", ""),
                separator=para["separator"],
            )
        )

    return TranslateResponse(paragraphs=paragraph_results)


@router.post("/translate-image-html", response_class=HTMLResponse)
async def translate_image_html(request: Request, file: UploadFile = File(...)):
    """HTMX endpoint for image translation"""
    try:
        file_bytes = await file.read()

        valid, error = validate_image_file(file_bytes, file.filename or "image.png")
        if not valid:
            return templates.TemplateResponse(
                request=request,
                name="fragments/error.html",
                context={"message": error},
            )

        extracted_text = await extract_text_from_image(file_bytes)

        if not extracted_text.strip():
            return templates.TemplateResponse(
                request=request,
                name="fragments/error.html",
                context={"message": "No Chinese text found in image"},
            )

        pipe = get_pipeline()

        # Split text into paragraphs
        paragraphs = split_into_paragraphs(extracted_text)

        # Process each paragraph through the pipeline
        paragraph_results = []
        for para in paragraphs:
            results = await pipe.aforward(para["content"])
            translations = [
                {"segment": seg, "pinyin": pinyin, "english": english}
                for seg, pinyin, english in results
            ]
            paragraph_results.append(
                {
                    "translations": translations,
                    "indent": para.get("indent", ""),
                    "separator": para["separator"],
                }
            )

        return templates.TemplateResponse(
            request=request,
            name="fragments/results.html",
            context={"paragraphs": paragraph_results, "original_text": extracted_text},
        )
    except Exception as e:
        return templates.TemplateResponse(
            request=request,
            name="fragments/error.html",
            context={"message": f"OCR error: {e}"},
        )
