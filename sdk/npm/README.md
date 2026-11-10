# dalibre-plugin

The tool for writing [Dalibre](https://github.com/djyde/dalibre) plugins:
start one from the template, run its functions as Dalibre does, check
and pack it.

```sh
npx dalibre-plugin init my-source --id com.example.books --name "Example Books"
npx dalibre-plugin check my-source
npx dalibre-plugin version
```

Each version is the Dalibre release of the same number: it holds plugins
to what that Dalibre can run. See [Writing Dalibre
plugins](https://github.com/djyde/dalibre/blob/main/docs/plugins/README.md).
