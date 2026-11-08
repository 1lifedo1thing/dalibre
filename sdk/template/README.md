# {{NAME}}

A [Dalibre](https://github.com/djyde/dalibre) source plugin.

```sh
dalibre-plugin check .                         # validate plugin.json and main.js
dalibre-plugin search . "dune"                 # run search as Dalibre would
dalibre-plugin download . "dune" --format EPUB # resolve and download the first result
dalibre-plugin pack .                          # make {{ID}}-<version>.dalibre-plugin
```

Pass settings with `--set apiKey=…`.
