"""
Admin routes for user profile and progress management.

Endpoints:
- GET /admin - Admin page
- POST /admin/profile - Update profile
- GET /admin/progress/export - Download progress JSON
- POST /admin/progress/import - Upload progress JSON
"""

import json
from datetime import datetime, timezone

from fastapi import APIRouter, Form, Request, UploadFile
from fastapi.responses import HTMLResponse, Response

from app.persistence.profile import (
    count_known_vocab,
    count_learning_vocab,
    count_total_vocab,
    get_user_profile,
    upsert_user_profile,
)
from app.persistence.progress_sync import (
    ImportError as ProgressImportError,
    export_progress_json,
    import_progress_json,
)
from app.templates_config import templates

router = APIRouter(prefix="/admin", tags=["admin"])

# Maximum upload size for progress file (1MB should be plenty)
MAX_PROGRESS_FILE_SIZE = 1 * 1024 * 1024


@router.get("", response_class=HTMLResponse)
async def admin_page(request: Request):
    """Render the admin page."""
    profile = get_user_profile()
    known_count = count_known_vocab()
    learning_count = count_learning_vocab()
    total_count = count_total_vocab()

    return templates.TemplateResponse(
        request=request,
        name="admin.html",
        context={
            "profile": profile,
            "known_count": known_count,
            "learning_count": learning_count,
            "total_count": total_count,
        },
    )


@router.post("/profile", response_class=HTMLResponse)
async def update_profile(
    request: Request,
    name: str = Form(""),
    email: str = Form(""),
    language: str = Form("zh-CN"),
):
    """Update user profile and return success fragment."""
    profile = upsert_user_profile(name=name, email=email, language=language)
    known_count = count_known_vocab()
    learning_count = count_learning_vocab()
    total_count = count_total_vocab()

    return templates.TemplateResponse(
        request=request,
        name="fragments/admin_profile.html",
        context={
            "profile": profile,
            "known_count": known_count,
            "learning_count": learning_count,
            "total_count": total_count,
            "success_message": "Profile updated successfully!",
        },
    )


@router.get("/progress/export")
async def export_progress():
    """Download learning progress as JSON file."""
    json_content = export_progress_json()
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")
    filename = f"language_app_progress_{timestamp}.json"

    return Response(
        content=json_content,
        media_type="application/json",
        headers={
            "Content-Disposition": f'attachment; filename="{filename}"',
        },
    )


@router.post("/progress/import", response_class=HTMLResponse)
async def import_progress(request: Request, file: UploadFile):
    """
    Import learning progress from uploaded JSON file.
    
    Overwrites existing vocab_items, srs_state, and vocab_lookups.
    """
    # Validate file size
    contents = await file.read()
    if len(contents) > MAX_PROGRESS_FILE_SIZE:
        return templates.TemplateResponse(
            request=request,
            name="fragments/admin_import_result.html",
            context={
                "success": False,
                "error": f"File too large. Maximum size is {MAX_PROGRESS_FILE_SIZE // 1024}KB.",
            },
        )

    # Validate file type
    if not file.filename or not file.filename.endswith(".json"):
        return templates.TemplateResponse(
            request=request,
            name="fragments/admin_import_result.html",
            context={
                "success": False,
                "error": "Invalid file type. Please upload a .json file.",
            },
        )

    try:
        json_str = contents.decode("utf-8")
        counts = import_progress_json(json_str)

        return templates.TemplateResponse(
            request=request,
            name="fragments/admin_import_result.html",
            context={
                "success": True,
                "counts": counts,
            },
        )
    except ProgressImportError as e:
        return templates.TemplateResponse(
            request=request,
            name="fragments/admin_import_result.html",
            context={
                "success": False,
                "error": str(e),
            },
        )
    except Exception as e:
        return templates.TemplateResponse(
            request=request,
            name="fragments/admin_import_result.html",
            context={
                "success": False,
                "error": f"Import failed: {e}",
            },
        )
