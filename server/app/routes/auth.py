"""
Authentication and homepage routes.

Endpoints:
- GET /login - Login page
- POST /login - Login form submission
- POST /logout - Logout
- GET / - Homepage
"""

import os

from fastapi import APIRouter, Form, Request
from fastapi.responses import HTMLResponse, RedirectResponse

from app.auth import get_session_manager, verify_password
from app.templates_config import templates

router = APIRouter(tags=["auth"])


@router.get("/login", response_class=HTMLResponse)
async def login_page(request: Request):
    """Serve the login page."""
    # If already authenticated, redirect to home
    session_token = request.cookies.get("session")
    if session_token and get_session_manager().verify_session(session_token):
        return RedirectResponse(url="/", status_code=303)
    return templates.TemplateResponse(request=request, name="login.html")


@router.post("/login", response_class=HTMLResponse)
async def login_submit(request: Request, password: str = Form(...)):
    """Handle login form submission."""
    if not verify_password(password):
        return templates.TemplateResponse(
            request=request,
            name="fragments/login_error.html",
            context={"message": "Invalid password"},
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
async def homepage(request: Request):
    """Serve the main page with the translation form"""
    vite_dev_server = os.getenv("VITE_DEV_SERVER")
    return templates.TemplateResponse(
        request=request,
        name="index.html",
        context={"vite_dev_server": vite_dev_server},
    )


@router.get("/translations", response_class=HTMLResponse)
async def translations_page(request: Request):
    """Serve the translations history page"""
    return templates.TemplateResponse(request=request, name="translations.html")
