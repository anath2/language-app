const pastelColors = [
  "#FFB3BA",
  "#BAFFC9",
  "#BAE1FF",
  "#FFFFBA",
  "#FFD9BA",
  "#E0BBE4",
  "#C9F0FF",
  "#FFDAB3"
];

export function getPastelColor(index: number): string {
  return pastelColors[index % pastelColors.length];
}

export function formatTimeAgo(isoString: string): string {
  const date = new Date(isoString);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (seconds < 60) return "Just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} hr ago`;
  return date.toLocaleDateString();
}
