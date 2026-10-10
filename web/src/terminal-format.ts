export type TextCell = string | { text: string; tone?: "dim" | "warn" | "crit" | "bright" };
export type FormattedCell = { text: string; tone?: "dim" | "warn" | "crit" | "bright" };

export function formatTextTable(headers: string[], rows: TextCell[][], right: number[] = []): FormattedCell[][] {
  const plain = (cell: TextCell | undefined) => typeof cell === "string" ? cell : cell?.text ?? "";
  const widths = headers.map((head, column) => Math.max(head.length, ...rows.map(row => plain(row[column]).length)));
  return [headers.map((head, column) => ({text: head.padEnd(widths[column]), tone: "dim" as const})),
    ...rows.map(row => headers.map((_, column) => {
      const cell = row[column] ?? "";
      const text = plain(cell);
      return {text: right.includes(column) ? text.padStart(widths[column]) : text.padEnd(widths[column]),
        tone: typeof cell === "string" ? undefined : cell.tone};
    }))];
}

export function blockBar(percent: number, width: number, pace?: number): {text: string; filled: boolean; marker: boolean}[] {
  const amount = Math.max(0, Math.min(width, Math.round(percent / 100 * width)));
  const marker = pace === undefined ? -1 : Math.max(0, Math.min(width - 1, Math.round(pace / 100 * width)));
  return Array.from({length: width}, (_, index) => ({
    text: index === marker ? "│" : index < amount ? "█" : "░",
    filled: index < amount,
    marker: index === marker,
  }));
}
