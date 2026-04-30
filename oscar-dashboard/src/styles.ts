/**
 * Colors with a number are sorted from lightest to darkest
 */
enum OscarColors {
  Green1 = "#B8CEB8",
  Green2 = "#9AB99A",
  Green3 = "#95B699",
  Green4 = "#009688",
  Blue = "#1F5FA6",
  Gray1 = "#E0E0E0",
  Gray2 = "#D9D9D9",
  DarkGrayText = "#757575",
  Red = "#EC221E",
}

export enum OscarStyles {
  border = `1px solid ${OscarColors.Gray2}`,
}

/**
 * @param color - hex color string
 * @param opacity - number between 0 and 1
 *
 * @returns color with opacity
 */
export function ColorWithOpacity(color: OscarColors | string, opacity: number) {
  return color + Math.round(255 * opacity).toString(16);
}

export default OscarColors;
