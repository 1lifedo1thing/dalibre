// @ts-check
// Project Gutenberg, through Gutendex (https://gutendex.com).

const API = "https://gutendex.com/books/";

// Gutendex's MIME types and the formats Dalibre names them by.
/** @type {Record<string, Dalibre.FormatName>} */
const FORMATS = {
  "application/epub+zip": "EPUB",
  "application/x-mobipocket-ebook": "MOBI",
  "application/pdf": "PDF",
  "text/plain; charset=utf-8": "TXT",
  "text/plain; charset=us-ascii": "TXT",
};

// Gutendex writes authors as "Austen, Jane".
/** @param {string} name */
function author(name) {
  const [last, first] = name.split(", ");
  return first ? `${first} ${last}` : name;
}

/**
 * @param {Dalibre.Query} query
 * @returns {Promise<Dalibre.Page>}
 */
export async function search({ text, page }) {
  let url = `${API}?search=${encodeURIComponent(text)}&page=${page}`;
  if (dalibre.settings.languages) {
    url += `&languages=${encodeURIComponent(dalibre.settings.languages.replace(/\s/g, ""))}`;
  }
  const res = await dalibre.fetch(url);
  if (res.status === 404) return { items: [] }; // past the last page
  if (res.status === 429) throw dalibre.error("rateLimited");
  if (!res.ok) throw dalibre.error("unavailable", `Gutendex answered ${res.status}`);
  const data = res.json();

  const items = data.results.map((/** @type {any} */ b) => {
    /** @type {Dalibre.Format[]} */
    const formats = [];
    for (const [mime, href] of Object.entries(b.formats)) {
      const format = FORMATS[mime];
      if (format && !formats.some((f) => f.format === format)) {
        formats.push({ format, url: String(href) });
      }
    }
    return {
      id: String(b.id),
      title: b.title,
      authors: b.authors.map((/** @type {any} */ a) => author(a.name)),
      language: b.languages[0],
      tags: b.subjects.slice(0, 5),
      identifiers: { gutenberg: String(b.id) },
      cover: b.formats["image/jpeg"],
      formats,
    };
  });
  // Some books are audio or images only.
  return { items: items.filter((b) => b.formats.length > 0), hasMore: Boolean(data.next) };
}
