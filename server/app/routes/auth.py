"""
Authentication and homepage routes.

Endpoints:
- GET /login - Login page (serves SPA)
- POST /login - Login form submission
- POST /logout - Logout
- GET / - Homepage (serves SPA)
"""

import os
from pathlib import Path

from fastapi import APIRouter, Form, Request
from fastapi.responses import HTMLResponse, RedirectResponse

from app.auth import get_session_manager, verify_password

router = APIRouter(tags=["auth"])

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


@router.get("/login", response_class=HTMLResponse)
async def login_page(request: Request):
    """Serve the login page (SPA handles login UI)."""
    # If already authenticated, redirect to home
    session_token = request.cookies.get("session")
    if session_token and get_session_manager().verify_session(session_token):
        return RedirectResponse(url="/", status_code=303)
    return HTMLResponse(content=get_spa_html())


@router.post("/login")
async def login_submit(password: str = Form(...)):
    """Handle login form submission."""
    if not verify_password(password):
        return HTMLResponse(
            content="Invalid password",
            status_code=401,
        )

    response = RedirectResponse(url="/", status_code=303)
    get_session_manager().set_session_cookie(response)
    return response


@router.post("/logout")
async def logout():
    """Clear session cookie and redirect to login."""
    response = RedirectResponse(url="/login", status_code=303)
    get_session_manager().clear_session_cookie(response)
    return response


@router.get("/", response_class=HTMLResponse)
async def homepage():
    """Serve the SPA."""
    return HTMLResponse(content=get_spa_html())


@router.get("/translations", response_class=HTMLResponse)
async def translations_page():
    """Serve the SPA (router handles /translations view)."""
    return HTMLResponse(content=get_spa_html())
