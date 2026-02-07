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
