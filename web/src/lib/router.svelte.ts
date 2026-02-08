type HomeRoute = { page: "home" };
type TranslationRoute = { page: "translation"; id: string };
type VocabRoute = { page: "vocab" };
type AdminRoute = { page: "admin" };
type LoginRoute = { page: "login"; returnUrl?: string };
type Route = HomeRoute | TranslationRoute | VocabRoute | AdminRoute | LoginRoute;

class Router {
  route = $state<Route>({ page: "home" });

  constructor() {
    // Handle hash-based navigation for login to avoid backend auth issues
    const hash = window.location.hash;
    if (hash.startsWith('#/login')) {
      const returnUrl = new URLSearchParams(window.location.search).get('return');
      this.route = { page: "login", returnUrl: returnUrl || '/' };
    } else {
      this.route = this.parsePath(window.location.pathname);
    }

    window.addEventListener("popstate", () => {
      const hash = window.location.hash;
      if (hash.startsWith('#/login')) {
        const returnUrl = new URLSearchParams(window.location.search).get('return');
        this.route = { page: "login", returnUrl: returnUrl || '/' };
      } else {
        this.route = this.parsePath(window.location.pathname);
      }
    });
  }

  navigateTo(id: string) {
    this.route = { page: "translation", id };
    history.pushState(null, "", `/translations/${id}`);
  }

  navigateHome() {
    this.route = { page: "home" };
    history.pushState(null, "", "/");
  }

  navigateToVocab() {
    this.route = { page: "vocab" };
    history.pushState(null, "", "/vocab");
  }

  navigateToAdmin() {
    this.route = { page: "admin" };
    history.pushState(null, "", "/admin");
  }

  navigateToLogin(returnUrl?: string) {
    this.route = { page: "login", returnUrl };
    if (returnUrl) {
      window.location.href = `#/login?return=$${encodeURIComponent(returnUrl)}`;
    } else {
      window.location.href = `#/login`;
    }
  }

  navigateToLoginWithHash(login: boolean) {
    if (login) {
      history.replaceState(null, "", "#/login");
      this.route = { page: "login" };
    } else {
      history.back();
      if (window.location.pathname === '/' && !window.location.hash) {
        this.route = { page: "home" };
      }
    }
  }

  private parsePath(pathname: string): Route {
    if (pathname === "/vocab") return { page: "vocab" };
    if (pathname === "/admin") return { page: "admin" };
    const match = pathname.match(/^\/translations\/([^/]+)\/?$/);
    if (match) return { page: "translation", id: match[1] };
    return { page: "home" };
  }
}

export const router = new Router();
