// Type declarations for Dalibre plugins, plugin API 1.
//
// Plugins are ES modules (ES2023) that export search and, optionally,
// resolve; CommonJS (module.exports) works too. In plain JavaScript, put
// this file next to main.js with the template's jsconfig.json and add
// `// @ts-check` to main.js: editors then check your code and complete
// the API. In TypeScript, compile to one module, for example with
// `esbuild main.ts --bundle --format=esm --target=es2023 --outfile=main.js`.

declare namespace Dalibre {
  /** What search is asked for. */
  interface Query {
    /** What the user typed, trimmed. May be empty: return popular or recent books, or nothing. */
    text: string;
    /** The page asked for, from 1. */
    page: number;
    /** The cursor the previous page returned; "" on the first page. */
    cursor: string;
  }

  /** A page of results. */
  interface Page {
    /** At most 100. Return [] when nothing is found. */
    items: Result[];
    /** Whether the user can ask for the next page. */
    hasMore?: boolean;
    /** Handed back as query.cursor for the next page, for APIs that page by token. */
    cursor?: string;
  }

  /** A book the source found. Only id, title and formats are required. */
  interface Result {
    /** Unique within this source, at most 256 characters. */
    id: string;
    title: string;
    authors?: string[];
    /** Plain text or simple HTML. */
    description?: string;
    publisher?: string;
    /** ISO 639: "en", "eng", "zh-CN". */
    language?: string;
    /** "2006", "2006-01", "2006-01-02" or an RFC 3339 time. */
    pubdate?: string;
    series?: string;
    seriesIndex?: number;
    tags?: string[];
    /** As calibre keeps them: { isbn: "…", goodreads: "…" }. Use your plugin's id as a key for its own id. */
    identifiers?: Record<string, string>;
    /** An image URL on a host in permissions.hosts. */
    cover?: string;
    /** At least one. */
    formats: Format[];
    /** Anything JSON; handed back unchanged to resolve. */
    extra?: unknown;
  }

  type FormatName =
    | "EPUB" | "KEPUB" | "AZW3" | "AZW" | "MOBI" | "PDF" | "FB2" | "DJVU" | "CBZ" | "CBR" | "CB7"
    | "TXT" | "RTF" | "DOCX" | "ODT" | "HTMLZ" | "LIT" | "PRC" | "PDB" | "LRF" | "SNB" | "TCR";

  interface Format {
    format: FormatName;
    /** Bytes, if known; shown to the user. */
    size?: number;
    /** Where to download it. Leave it out and export resolve when links must be made at download time. */
    url?: string;
  }

  /** Where the host downloads a file from. */
  interface DownloadRequest {
    url: string;
    headers?: Record<string, string>;
    /** The file's name; else the server's, else the title. The extension is fixed to the format. */
    filename?: string;
  }

  interface FetchInit {
    method?: string;
    headers?: Record<string, string>;
    body?: string;
  }

  interface Response {
    status: number;
    statusText: string;
    ok: boolean;
    /** The final URL, after redirects. */
    url: string;
    /** Lowercase names. */
    headers: Record<string, string>;
    text(): string;
    json(): any;
  }

  /** Kinds the app tells the user about differently. */
  type ErrorKind =
    | "auth"        // credentials missing or refused: the app offers the plugin's settings
    | "notFound"    // the book or file is gone
    | "rateLimited" // the service asks to slow down
    | "unavailable" // the service is down
    | "invalid";    // the query can't be searched

  /** What a source plugin exports. */
  interface Source {
    search(query: Query): Page | Promise<Page>;
    resolve?(item: Result, format: FormatName): DownloadRequest | Promise<DownloadRequest>;
  }
}

declare const dalibre: {
  /** The plugin API version of the running Dalibre. */
  readonly api: number;
  readonly plugin: { readonly id: string; readonly version: string };
  /** The user's values for the settings in plugin.json, by key; defaults when not set. */
  readonly settings: Readonly<Record<string, string>>;
  /**
   * The web's fetch, cut down. Only hosts in permissions.hosts, redirects
   * included; at most 50 requests per call and 16 MB per response. Requests
   * run one at a time, so Promise.all doesn't make them parallel.
   */
  fetch(url: string, init?: Dalibre.FetchInit): Promise<Dalibre.Response>;
  /** An Error the app understands: `throw dalibre.error("auth", "The API key was refused")`. */
  error(kind: Dalibre.ErrorKind, message?: string): Error;
  log(...args: unknown[]): void;
};

declare const console: {
  log(...args: unknown[]): void;
  info(...args: unknown[]): void;
  warn(...args: unknown[]): void;
  error(...args: unknown[]): void;
  debug(...args: unknown[]): void;
};

/** For CommonJS plugins; ES modules export search and resolve instead. */
declare const module: { exports: Partial<Dalibre.Source> };
declare const exports: Partial<Dalibre.Source>;
