type HomeRoute = { page: "home" };
type TranslationRoute = { page: "translation"; id: string };
type VocabRoute = { page: "vocab" };
type Route = HomeRoute | TranslationRoute | VocabRoute;

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

  private parsePath(pathname: string): Route {
    if (pathname === "/vocab") return { page: "vocab" };
    const match = pathname.match(/^\/translations\/([^/]+)\/?$/);
    if (match) return { page: "translation", id: match[1] };
    return { page: "home" };
  }
}

export const router = new Router();
