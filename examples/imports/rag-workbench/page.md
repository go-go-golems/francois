## What it is

RAG Workbench exposes a retrieval pipeline as an editable workspace. Start with the included public documentation, upload Markdown, run lexical retrieval, and inspect which passages are selected before asking the model to produce an answer.

The bundled sample corpus works without a separate search service. Hosted model requests use the site's shared limits.

## Why we made it

Retrieval settings affect which source material reaches a model. Showing the source passages and intermediate results makes those decisions inspectable while the pipeline is being changed.

## Related

- [Tabula Workbench](/apps/tabula-workbench) — reactive inputs and visual results.

## Sources

The built-in sample corpus is distributed with this experiment as `demo-corpus.json`. Inspect each sample document's source information before using it as a reference.
