// Authentication state and methods

class AuthStore {
  isAuthenticated = $state(false);
  isLoading = $state(true);
  error = $state<string>('');

  /**
   * Checks the current authentication status by hitting a lightweight authenticated endpoint.
   */
  async checkAuthStatus(): Promise<void> {
    try {
      const response = await fetch('/api/review/words/count', {
        method: 'GET',
        credentials: 'include',
      });

      if (response.ok) {
        this.isAuthenticated = true;
      } else if (response.status === 401) {
        this.isAuthenticated = false;
      } else {
        console.error('Unexpected response checking auth status:', response.status);
        this.isAuthenticated = false;
      }
    } catch (error) {
      console.error('Error checking auth status:', error);
      this.isAuthenticated = false;
    } finally {
      this.isLoading = false;
    }
  }

  /**
   * Attempts to log in with the provided password.
   */
  async login(password: string): Promise<boolean> {
    try {
      this.error = '';
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
        credentials: 'include',
      });

      if (response.ok) {
        this.isAuthenticated = true;
        return true;
      } else if (response.status === 401) {
        this.error = 'Invalid password';
        return false;
      } else {
        this.error = 'Login failed. Please try again.';
        return false;
      }
    } catch (err) {
      console.error('Error during login:', err);
      this.error = 'Network error. Please check your connection.';
      return false;
    }
  }

  /**
   * Logs out the current user.
   */
  async logout(): Promise<void> {
    try {
      await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
    } catch (error) {
      console.error('Error during logout:', error);
    } finally {
      this.isAuthenticated = false;
      window.location.href = '/';
    }
  }
}

export const auth = new AuthStore();
