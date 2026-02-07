type HomeRoute = { page: "home" };
type TranslationRoute = { page: "translation"; id: string };
type Route = HomeRoute | TranslationRoute;

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

  private parsePath(pathname: string): Route {
    const match = pathname.match(/^\/translations\/([^/]+)\/?$/);
    if (match) return { page: "translation", id: match[1] };
    return { page: "home" };
  }
}

export const router = new Router();
