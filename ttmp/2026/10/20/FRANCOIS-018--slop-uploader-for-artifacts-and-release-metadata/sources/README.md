# Input and evidence inventory

- `slop-uploader.jpeg`: supplied visual reference, copied unchanged from `~/Downloads`.
- `slop-uploader.zip`: supplied standalone Node/plain-JavaScript prototype, copied unchanged.
- `prototype/slop-uploader/`: exact extraction after checking member paths for traversal.
- `input-manifest.json`: original input sizes and SHA-256 hashes.
- `analysis-evidence.json`: bounded helper-test result and manifest-loss experiment.

The original JPEG shows only the upper portion of the uploader; the ZIP supplies the full form. No source archive dependencies were installed and no real token was loaded. The pure helper test was run; proxy/browser/end-to-end tests were not run. The guide was checked against local repository revision `fbb4f72ffd3917bf6e4f2229c5043a9b176b29ea`, not a production snapshot.
