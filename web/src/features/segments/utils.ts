const pastelColors = [
  '#FFB3BA',
  '#BAFFC9',
  '#BAE1FF',
  '#FFFFBA',
  '#FFD9BA',
  '#E0BBE4',
  '#C9F0FF',
  '#FFDAB3',
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
