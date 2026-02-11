const pastelColors = [
  '#C2E0D3', // celadon / jade green
  '#E4C6BD', // cinnabar / terracotta pink
  '#B8CEE8', // indigo / soft blue
  '#E0D6B4', // amber / golden sand
  '#D1BDE0', // plum / soft purple
  '#E8D4B0', // persimmon / warm orange
  '#B0D8D0', // teal / cool green
  '#D0C4E0', // iris / lavender
];

/**
 * Returns a pastel color based on the provided index.
 * Colors cycle through a predefined palette of 8 pastel colors.
 * @param index - The index to determine which color to return
 * @returns A hex color string from the pastel palette
 */
export function getPastelColor(index: number): string {
  return pastelColors[index % pastelColors.length];
}
