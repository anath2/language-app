"""
Admin routes for user profile and progress management.

Endpoints:
- GET /admin - Admin page (serves SPA)
- GET /admin/progress/export - Download progress JSON
- POST /admin/progress/import - Upload progress JSON
"""

import os
from datetime import datetime, timezone
from pathlib import Path

from fastapi import APIRouter, HTTPException, UploadFile, Form, Body
from fastapi.responses import HTMLResponse, JSONResponse, Response

from app.persistence.progress_sync import (
    ImportError as ProgressImportError,
    export_progress_json,
    import_progress_json,
)
from app.persistence.profile import (
    get_user_profile,
    upsert_user_profile,
    count_known_vocab,
    count_learning_vocab,
    count_total_vocab,
)

router = APIRouter(prefix="/admin", tags=["admin"])

# SPA HTML content
BASE_DIR = Path(__file__).resolve().parent.parent
ROOT_DIR = BASE_DIR.parent.parent
WEB_DIST_DIR = ROOT_DIR / "web" / "dist"
VITE_DEV_SERVER = os.getenv("VITE_DEV_SERVER")


def get_spa_html() -> str:
    """Get SPA HTML - from Vite dev server in dev, or built files in production."""
    if VITE_DEV_SERVER:
        # Development - serve from Vite dev server
        return f"""<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Language App</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="{VITE_DEV_SERVER}/src/main.ts"></script>
  </body>
</html>
"""
    else:
        # Production - serve built files
        try:
            return (WEB_DIST_DIR / "index.html").read_text()
        except FileNotFoundError:
            return "<!DOCTYPE html><html><body>App not built. Run `cd web && npm run build`</body></html>"


@router.get("", response_class=HTMLResponse)
async def admin_page():
    """Serve the admin page (SPA handles admin UI)."""
    return HTMLResponse(content=get_spa_html())


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


# Maximum upload size for progress file (1MB should be plenty)
MAX_PROGRESS_FILE_SIZE = 1 * 1024 * 1024


@router.post("/progress/import")
async def import_progress(file: UploadFile):
    """
    Import learning progress from uploaded JSON file.

    Overwrites existing vocab_items, srs_state, and vocab_lookups.
    """
    # Validate file size
    contents = await file.read()
    if len(contents) > MAX_PROGRESS_FILE_SIZE:
        raise HTTPException(
            status_code=400,
            detail=f"File too large. Maximum size is {MAX_PROGRESS_FILE_SIZE // 1024}KB.",
        )

    # Validate file type
    if not file.filename or not file.filename.endswith(".json"):
        raise HTTPException(
            status_code=400,
            detail="Invalid file type. Please upload a .json file.",
        )

    try:
        json_str = contents.decode("utf-8")
        counts = import_progress_json(json_str)
        return JSONResponse(content={"success": True, "counts": counts})
    except ProgressImportError as e:
        raise HTTPException(status_code=400, detail=str(e)) from e
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Import failed: {e}") from e


@router.get("/api/profile")
async def get_profile():
    """Get user profile and vocabulary statistics."""
    profile = get_user_profile()
    vocab_stats = {
        "known": count_known_vocab(),
        "learning": count_learning_vocab(),
        "total": count_total_vocab(),
    }
    profile_dict = None
    if profile:
        profile_dict = {
            "name": profile.name,
            "email": profile.email,
            "language": profile.language,
            "created_at": profile.created_at,
            "updated_at": profile.updated_at,
        }
    return JSONResponse(content={"profile": profile_dict, "vocabStats": vocab_stats})


@router.post("/api/profile")
async def update_profile(name: str = Form(...), email: str = Form(...), language: str = Form(...)):
    """Update user profile."""
    from datetime import datetime
    updated_profile = upsert_user_profile(name=name, email=email, language=language)
    profile_dict = {
        "name": updated_profile.name,
        "email": updated_profile.email,
        "language": updated_profile.language,
        "created_at": updated_profile.created_at,
        "updated_at": updated_profile.updated_at,
    }
    return JSONResponse(content={"profile": profile_dict})
