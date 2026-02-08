// Authentication state and methods
// This module manages the authentication state for the Svelte 5 SPA
// It handles login, logout, and session checking

const API_BASE = import.meta.env.DEV ? '/api' : `http://${window.location.host}/api`;

// Authentication state
let isAuthenticated = $state(false);
let isLoading = $state(true);
let error = $state<string>('');

/**
 * Checks the current authentication status by attempting to access the API.
 * Updates the reactive authentication state based on the response.
 */
export async function checkAuthStatus(): Promise<void> {
  try {
    const response = await fetch(`${API_BASE}/texts`, {
      method: 'GET',
      credentials: 'include',
    });

    if (response.ok) {
      isAuthenticated = true;
    } else if (response.status === 401) {
      isAuthenticated = false;
    } else {
      console.error('Unexpected response checking auth status:', response.status);
      isAuthenticated = false;
    }
  } catch (error) {
    console.error('Error checking auth status:', error);
    isAuthenticated = false;
  } finally {
    isLoading = false;
  }
}

/**
 * Attempts to log in with the provided password.
 * @param password - The password to use for authentication
 * @returns Promise resolving to true if login successful, false otherwise
 * @throws Network error if the request fails
 */
export async function login(password: string): Promise<boolean> {
  try {
    error = '';
    const formData = new URLSearchParams();
    formData.append('password', password);

    const response = await fetch('/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      body: formData.toString(),
      credentials: 'include',
    });

    if (response.ok) {
      isAuthenticated = true;
      return true;
    } else if (response.status === 401) {
      error = 'Invalid password';
      return false;
    } else {
      error = 'Login failed. Please try again.';
      return false;
    }
  } catch (error) {
    console.error('Error during login:', error);
    error = 'Network error. Please check your connection.';
    return false;
  }
}

/**
 * Logs out the current user and redirects to the home page.
 * Updates the reactive authentication state to logged out.
 */
export async function logout(): Promise<void> {
  try {
    await fetch('/logout', {
      method: 'POST',
      credentials: 'include',
    });
  } catch (error) {
    console.error('Error during logout:', error);
  } finally {
    isAuthenticated = false;
    window.location.href = '/';
  }
}

// Export reactive state
export const auth = {
  get isAuthenticated() {
    return isAuthenticated;
  },
  get isLoading() {
    return isLoading;
  },
  get error() {
    return error;
  },
  checkAuthStatus,
  login,
  logout,
};
