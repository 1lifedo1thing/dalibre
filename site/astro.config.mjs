import { defineConfig } from "astro/config";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://dalibre.randynamic.org",
  vite: { plugins: [tailwindcss()] },
  markdown: {
    shikiConfig: { theme: "vesper", wrap: false },
  },
});
