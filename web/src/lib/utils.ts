/**
 * Formats an ISO date string into a human-readable time ago format.
 * @param isoString - The ISO 8601 date string to format
 * @returns A string representing the time difference (e.g., "5 min ago", "2 hr ago", or a date string)
 */
export function formatTimeAgo(isoString: string): string {
  const date = new Date(isoString);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (seconds < 60) return 'Just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} hr ago`;
  return date.toLocaleDateString();
}
