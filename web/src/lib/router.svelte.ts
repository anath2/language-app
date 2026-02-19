/**
 * Router for the Svelte 5 SPA.
 * Handles navigation between different pages using pushState/popstate API.
 * Supports regular pathname routing and hash-based navigation for login.
 */

/** Route type definitions */
type HomeRoute = { page: 'home' };
type TranslationRoute = { page: 'translation'; id: string };
type VocabRoute = { page: 'vocab' };
type AdminRoute = { page: 'admin' };
type DiscoverRoute = { page: 'discover' };
type LoginRoute = { page: 'login'; returnUrl?: string };
type Route = HomeRoute | TranslationRoute | VocabRoute | AdminRoute | DiscoverRoute | LoginRoute;

/**
 * Router class managing navigation and route state.
 * Uses reactive state to trigger Svelte component updates.
 */
class Router {
  /** The current active route */
  route = $state<Route>({ page: 'home' });

  constructor() {
    // Handle hash-based navigation for login to avoid backend auth issues
    const hash = window.location.hash;
    if (hash.startsWith('#/login')) {
      const returnUrl = new URLSearchParams(window.location.search).get('return');
      this.route = { page: 'login', returnUrl: returnUrl || '/' };
    } else {
      this.route = this.parsePath(window.location.pathname);
    }

    window.addEventListener('popstate', () => {
      const hash = window.location.hash;
      if (hash.startsWith('#/login')) {
        const returnUrl = new URLSearchParams(window.location.search).get('return');
        this.route = { page: 'login', returnUrl: returnUrl || '/' };
      } else {
        this.route = this.parsePath(window.location.pathname);
      }
    });
  }

  /**
   * Navigate to a specific translation page.
   * @param id - The translation ID to navigate to
   */
  navigateTo(id: string) {
    this.route = { page: 'translation', id };
    history.pushState(null, '', `/translations/${id}`);
  }

  /**
   * Navigate to the home page.
   */
  navigateHome() {
    this.route = { page: 'home' };
    history.pushState(null, '', '/');
  }

  /**
   * Navigate to the vocabulary page.
   */
  navigateToVocab() {
    this.route = { page: 'vocab' };
    history.pushState(null, '', '/vocab');
  }

  /**
   * Navigate to the admin page.
   */
  navigateToAdmin() {
    this.route = { page: 'admin' };
    history.pushState(null, '', '/admin');
  }

  /**
   * Navigate to the discover/explore page.
   */
  navigateToDiscover() {
    this.route = { page: 'discover' };
    history.pushState(null, '', '/discover');
  }

  /**
   * Navigate to the login page with optional return URL.
   * Uses hash-based navigation for login to avoid backend auth issues.
   * @param returnUrl - The URL to redirect to after login
   */
  navigateToLogin(returnUrl?: string) {
    this.route = { page: 'login', returnUrl };
    if (returnUrl) {
      window.location.href = `#/login?return=$${encodeURIComponent(returnUrl)}`;
    } else {
      window.location.href = `#/login`;
    }
  }

  /**
   * Navigate to login page using hash-based navigation.
   * @param login - Whether to show login or go back
   */
  navigateToLoginWithHash(login: boolean) {
    if (login) {
      history.replaceState(null, '', '#/login');
      this.route = { page: 'login' };
    } else {
      history.back();
      if (window.location.pathname === '/' && !window.location.hash) {
        this.route = { page: 'home' };
      }
    }
  }

  /**
   * Parse the URL pathname and determine the route.
   * @param pathname - The URL pathname to parse
   * @returns The parsed route object
   * @private
   */
  private parsePath(pathname: string): Route {
    if (pathname === '/vocab') return { page: 'vocab' };
    if (pathname === '/admin') return { page: 'admin' };
    if (pathname === '/discover') return { page: 'discover' };
    const match = pathname.match(/^\/translations\/([^/]+)\/?$/);
    if (match) return { page: 'translation', id: match[1] };
    return { page: 'home' };
  }
}

/** Global router instance for the application */
export const router = new Router();
