type HomeRoute = { page: "home" };
type TranslationRoute = { page: "translation"; id: string };
type VocabRoute = { page: "vocab" };
type AdminRoute = { page: "admin" };
type Route = HomeRoute | TranslationRoute | VocabRoute | AdminRoute;

class Router {
  route = $state<Route>({ page: "home" });

  constructor() {
    this.route = this.parsePath(window.location.pathname);
    window.addEventListener("popstate", () => {
      this.route = this.parsePath(window.location.pathname);
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

  private parsePath(pathname: string): Route {
    if (pathname === "/vocab") return { page: "vocab" };
    if (pathname === "/admin") return { page: "admin" };
    const match = pathname.match(/^\/translations\/([^/]+)\/?$/);
    if (match) return { page: "translation", id: match[1] };
    return { page: "home" };
  }
}

export const router = new Router();
