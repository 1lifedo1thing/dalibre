// The Marketplace's plugins, read in the browser from the same
// marketplace.json Dalibre reads, so the plugins page lists what the app
// offers as soon as it changes.

export const url = "https://raw.githubusercontent.com/djyde/dalibre/refs/heads/marketplace/marketplace.json";

export interface Category {
  id: string;
  title: string;
}

interface Action {
  id: string;
  title: string;
  scope: "books" | "library";
}

export interface Plugin {
  id: string;
  name: string;
  version: string;
  description: string;
  author?: string;
  license?: string;
  icon?: string;
  homepage?: string;
  dalibre?: string;
  category: string;
  /** The calibre plugin it does the work of. */
  calibre?: string;
  /** calibre's downloads of that plugin, which orders the list. */
  downloads?: number;
  repo?: string;
  capabilities: {
    bookSource?: unknown;
    metadata?: { fields?: string[] };
    fileType?: { title?: string };
    actions?: Action[];
  };
  permissions: { hosts?: string[]; localNetwork?: boolean; library?: "read" | "write" };
}

export async function loadMarketplace(): Promise<{ categories: Category[]; plugins: Plugin[] }> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`fetching the Marketplace: ${res.status} ${res.statusText}`);
  const data = (await res.json()) as { categories: Category[]; plugins: Plugin[] };
  return { categories: data.categories ?? [], plugins: data.plugins ?? [] };
}

/** What the plugin adds to Dalibre, in the words of the app's menus. */
export function adds(p: Plugin): string[] {
  const out: string[] = [];
  const c = p.capabilities;
  if (c.bookSource) out.push("A source in Get books");
  if (c.metadata) {
    const f = c.metadata.fields;
    out.push(f?.length === 1 && f[0] === "cover" ? "Covers in Fetch metadata" : "A source in Fetch metadata");
  }
  if (c.fileType) out.push(c.fileType.title ?? "Works on the files you add");
  for (const a of c.actions ?? []) {
    const title = a.title.replace(/…$/, "");
    if (!out.includes(title)) out.push(title);
  }
  return out;
}

/** What the plugin asks the user to allow, in a few words. */
export function asks(p: Plugin): string[] {
  const out: string[] = [];
  const hosts = p.permissions.hosts ?? [];
  if (hosts.length) out.push(`Reaches ${hosts.join(", ")}`);
  if (p.permissions.localNetwork) out.push("Reaches your local network");
  if (p.permissions.library === "write") out.push("Reads and changes your library");
  else if (p.permissions.library === "read") out.push("Reads your library");
  if (!hosts.length && !p.permissions.localNetwork) out.push("Nothing on the internet");
  return out;
}
