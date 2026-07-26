// Locale-aware cell formatting via Intl, fed by hostContext locale/timeZone.

let currentLocale: string | undefined;
let currentTimeZone: string | undefined;

export function setLocale(locale?: string, timeZone?: string): void {
  currentLocale = locale || undefined;
  currentTimeZone = timeZone || undefined;
}

export function getLocale(): string | undefined {
  return currentLocale;
}

/** The host's time zone, when it named one. The calendar needs it to know
 * which day is today: the server's midnight is not the reader's. */
export function getTimeZone(): string | undefined {
  return currentTimeZone;
}

/**
 * Formats a cell value by column type and format spec.
 * Number formats: "int" | "decimal:<digits>" | "percent" | "currency:<code>".
 * Date formats: "date" | "datetime" | "time" | "relative".
 * Unknown/absent formats fall back to sensible defaults; malformed values
 * render as their string form rather than throwing.
 */
export function formatCell(value: unknown, type: string, format?: string): string {
  if (value === null || value === undefined) return "";
  try {
    switch (type) {
      case "number":
        return formatNumber(value, format);
      case "date":
        return formatDate(value, format);
      default:
        return String(value);
    }
  } catch {
    return String(value);
  }
}

function formatNumber(value: unknown, format?: string): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return String(value);

  const opts: Intl.NumberFormatOptions = {};
  if (format === "int") {
    opts.maximumFractionDigits = 0;
  } else if (format?.startsWith("decimal:")) {
    const digits = Number(format.slice("decimal:".length));
    if (Number.isInteger(digits) && digits >= 0 && digits <= 20) {
      opts.minimumFractionDigits = digits;
      opts.maximumFractionDigits = digits;
    }
  } else if (format === "percent") {
    opts.style = "percent";
    opts.maximumFractionDigits = 1;
  } else if (format?.startsWith("currency:")) {
    opts.style = "currency";
    opts.currency = format.slice("currency:".length);
  }
  return new Intl.NumberFormat(currentLocale, opts).format(n);
}

function formatDate(value: unknown, format?: string): string {
  const d = toDate(value);
  if (!d) return String(value);

  if (format === "relative") {
    return formatRelative(d);
  }
  const opts: Intl.DateTimeFormatOptions = { timeZone: currentTimeZone };
  switch (format) {
    case "time":
      opts.timeStyle = "short";
      break;
    case "datetime":
      opts.dateStyle = "medium";
      opts.timeStyle = "short";
      break;
    default: // "date" and unspecified
      opts.dateStyle = "medium";
  }
  return new Intl.DateTimeFormat(currentLocale, opts).format(d);
}

function toDate(value: unknown): Date | null {
  if (value instanceof Date) return value;
  if (typeof value === "number") return new Date(value);
  if (typeof value === "string") {
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? null : d;
  }
  return null;
}

function formatRelative(d: Date): string {
  const rtf = new Intl.RelativeTimeFormat(currentLocale, { numeric: "auto" });
  const diffMs = d.getTime() - Date.now();
  const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
    ["year", 1000 * 60 * 60 * 24 * 365],
    ["month", 1000 * 60 * 60 * 24 * 30],
    ["day", 1000 * 60 * 60 * 24],
    ["hour", 1000 * 60 * 60],
    ["minute", 1000 * 60],
  ];
  for (const [unit, ms] of units) {
    if (Math.abs(diffMs) >= ms) {
      return rtf.format(Math.trunc(diffMs / ms), unit);
    }
  }
  return rtf.format(Math.trunc(diffMs / 1000), "second");
}
