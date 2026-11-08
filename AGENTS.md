# Agents

## Design

UI is designed in Figma first, then built: the design is the spec.

- File: [Dalibre](https://www.figma.com/design/tZg6AMrNXoz9VYoa1xZoCo/Dalibre?node-id=22-4140&m=dev)
  (file key `tZg6AMrNXoz9VYoa1xZoCo`).
- New screens go into that file, built from its components and variables;
  `internal/app/theme` and `internal/app/widgets` mirror them in code.
- Don't mock UI in HTML or elsewhere; design it in the Figma file.
